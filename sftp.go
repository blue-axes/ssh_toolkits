package main

import (
	"fmt"
	sshd "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

type (
	SFTP struct {
		sess sshd.Session
	}
)

func NewSFTPServer() *SFTP {
	s := &SFTP{}
	return s
}

func (h *SFTP) Handle(sess sshd.Session) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	h.sess = sess
	defer h.Close()
	defer fmt.Println("end sftp.........")
	fmt.Println("start sftp...........")
	srv, err := sftp.NewServer(h.sess, sftp.WithServerWorkingDirectory("/")) // 跟目录
	if err != nil {
		h.Reply(1, err.Error())
		return
	}
	err = srv.Serve()
	if err != nil {
		h.Reply(1, err.Error())
		return
	}
	h.Reply(0, "")
}

func (h *SFTP) Reply(code int, msg string) {
	switch code {
	case 0:
		h.sess.Exit(0)
	default:
		h.sess.Write([]byte(msg))
		h.sess.Exit(code)
	}
}

func (h *SFTP) Close() {
	if h.sess != nil {
		h.sess.Close()
		h.sess = nil
	}
}
