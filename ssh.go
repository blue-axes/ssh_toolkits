package main

import (
	"fmt"
	"github.com/creack/pty"
	sshd "github.com/gliderlabs/ssh"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

type (
	SSH struct {
	}
)

func (h *SSH) Handle(sess sshd.Session) {
	defer sess.Close()
	switch sess.Subsystem() {
	case "sftp":
		NewSFTPServer().Handle(sess)
		return
	}
	if strings.HasPrefix(sess.RawCommand(), "scp ") {
		NewSCPServer().Handle(sess)
		return
	}
	cmdList := sess.Command()
	if len(cmdList) > 0 { // exec
		cmd := exec.Command(cmdList[0], cmdList[1:]...)
		cmd.Stdout = sess
		cmd.Stderr = sess
		cmd.Stdin = sess
		err := cmd.Run()
		if err != nil {
			sess.Write([]byte(err.Error()))
			sess.Exit(1)
			return
		}
		return
	}

	_, _, isPty := sess.Pty()
	if isPty && runtime.GOOS != "windows" { // pty
		ptmx, err := pty.Start(exec.Command("bash"))
		if err != nil {
			sess.Write([]byte(err.Error()))
			sess.Exit(1)
			return
		}
		defer ptmx.Close()
		handleResize(ptmx)
		go func() {
			_, _ = io.Copy(ptmx, sess)
		}()
		_, _ = io.Copy(sess, ptmx)
		return
	}

	if len(cmdList) == 0 { // exec
		switch runtime.GOOS {
		case "windows":
			cmdList = append(cmdList, "powershell")
		default:
			cmdList = append(cmdList, "bash")
		}
	}

	fmt.Println("exec: " + strings.Join(cmdList, " "))
	cmd := exec.Command(cmdList[0], cmdList[1:]...)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if runtime.GOOS == "windows" {
		out = io.NopCloser(simplifiedchinese.GB18030.NewDecoder().Reader(out))
	}
	go func() {
		_, _ = io.Copy(in, sess)
	}()
	go func() {
		_, _ = io.Copy(sess, out)
	}()

	err := cmd.Run()
	if err != nil {
		sess.Write([]byte(err.Error()))
		sess.Exit(1)
		return
	}

}
