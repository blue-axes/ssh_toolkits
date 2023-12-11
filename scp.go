package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	sshd "github.com/gliderlabs/ssh"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	SCPActionUpload   SCPAction = 1
	SCPActionDownload SCPAction = 2
)
const (
	SCPCodeOK ScpReplyCode = iota
	SCPCodeWarning
	SCPCodeFault
)

type (
	SCP struct {
		rw io.ReadWriter
	}
	T struct {
		AccessTimestamp int64
		ModifyTimestamp int64
	}
	C struct {
		Mode fs.FileMode
		Name string
		Size uint64
	}
	D struct {
		Mode fs.FileMode
		Name string
	}

	SCPAction    = int
	ScpReplyCode = uint8

	SCPCommand struct {
		Action       SCPAction
		Recursive    bool
		ShowVerbose  bool
		KeepMetaInfo bool
		Destination  string
	}
)

func NewSCPServer() *SCP {
	s := &SCP{}
	return s
}

func (h *SCP) Handle(sess sshd.Session) {
	defer sess.Close()
	fmt.Println(sess.Command())
	//cmd := exec.Command("scp", sess.Command()[1:]...)
	//rw := RW{
	//	r: sess,
	//	w: sess,
	//}
	//cmd.Stdin = rw
	//cmd.Stdout = rw
	//cmd.Stderr = rw
	//cmd.Run()
	//return

	//h.rw = RW{ // 调试时使用
	//    r: sess,
	//    w: sess,
	//}
	h.rw = sess

	command := h.parseCmd(sess)
	switch command.Action {
	case SCPActionUpload:
		h.upload(command)
	case SCPActionDownload:
		h.download(command)
	}
}

func (h *SCP) parseCmd(sess sshd.Session) (info SCPCommand) {
	var (
		isUpload   = false
		isDownload = false
	)
	command := sess.Command()
	commandLine := flag.NewFlagSet(command[0], flag.ContinueOnError)
	commandLine.BoolVar(&info.Recursive, "r", false, "is recursive")
	commandLine.BoolVar(&isUpload, "t", false, "is upload")
	commandLine.BoolVar(&isDownload, "f", false, "is download")
	commandLine.BoolVar(&info.ShowVerbose, "v", false, "show verbose")
	commandLine.BoolVar(&info.KeepMetaInfo, "p", false, "keep file meta info")
	commandLine.Parse(command[1:])

	dest := command[len(command)-1]
	if isUpload {
		info.Action = SCPActionUpload
	} else {
		info.Action = SCPActionDownload
	}
	info.Destination = dest
	return info
}

func (h *SCP) upload(command SCPCommand) {
	fmt.Println("upload ")
	if command.Recursive { // 传的是文件夹
		_ = os.Mkdir(path.Clean(command.Destination), 755)
	}
	h.replyOK() // 回复OK

	// 包装reader
	r := bufio.NewReader(h.rw)
	err := h.dealUpload(r, nil, command.Recursive, path.Clean(command.Destination))
	if err == nil {
		h.replyOK()
	}
}

func (h *SCP) dealUpload(r *bufio.Reader, t *T, recursive bool, prefixDir string) error {
	instruction, err := r.ReadByte()
	if err != nil {
		h.reply(SCPCodeFault, err.Error())
		return err
	}
	switch instruction {
	case 'T':
		tInstruction, err := h.parseT(r)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		h.replyOK()
		return h.dealUpload(r, tInstruction, recursive, prefixDir)
	case 'C':
		cInstruction, err := h.parseC(r)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		h.replyOK()
		// 读取文件内容，写入文件
		if !recursive { // 上传的是单个文件
			info, err := os.Stat(prefixDir)
			if errors.Is(err, os.ErrNotExist) || (info != nil && !info.IsDir()) { // 文件不存在或者不是一个文件夹，则当成更名处理
				basename := path.Base(prefixDir)
				prefixDir = path.Dir(prefixDir)
				cInstruction.Name = basename
			}
		}
		err = h.writeFile(r, t, cInstruction, prefixDir)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		h.replyOK()
		if !recursive { // 上传的是单个文件则返回
			return nil
		}
		return h.dealUpload(r, nil, recursive, prefixDir)
	case 'D':
		dInstruction, err := h.parseD(r)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		h.replyOK()
		// 创建文件夹
		err = h.createDirectory(t, dInstruction, prefixDir)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		return h.dealUpload(r, nil, recursive, path.Join(prefixDir, dInstruction.Name))
	case 'E':
		err = h.parseE(r)
		if err != nil {
			h.reply(SCPCodeFault, err.Error())
			return err
		}
		h.replyOK()
		return nil
	default:
		msg := fmt.Sprintf("unknown instruction :%c", instruction)
		h.reply(SCPCodeFault, msg)
		return errors.New(msg)
	}

}

func (h *SCP) parseT(r *bufio.Reader) (*T, error) {
	var (
		res = &T{}
	)
	data, err := r.ReadSlice(' ') // modify_time
	if err != nil {
		return res, err
	}
	res.ModifyTimestamp, err = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return res, errors.New(fmt.Sprintf("parse access_time error:%s", err.Error()))
	}
	data, err = r.ReadSlice(' ') //忽略第二段
	if err != nil {
		return res, err
	}
	data, err = r.ReadSlice(' ') //access_time
	res.AccessTimestamp, err = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return res, errors.New(fmt.Sprintf("parse modify_time error:%s", err.Error()))
	}
	data, err = r.ReadSlice('\n') //忽略第四段
	if err != nil {
		return res, err
	}
	return res, nil
}

func (h *SCP) parseC(r *bufio.Reader) (*C, error) {
	var (
		res = &C{}
	)
	data, err := r.ReadSlice(' ') // 权限位
	if err != nil {
		return res, err
	}
	mode, err := strconv.ParseInt(strings.TrimSpace(string(data)), 8, 64)
	if err != nil {
		return res, errors.New(fmt.Sprintf("parse file mode error:%s", err.Error()))
	}
	res.Mode = os.FileMode(mode)
	data, err = r.ReadSlice(' ') // 大小
	if err != nil {
		return res, err
	}
	res.Size, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return res, errors.New(fmt.Sprintf("parse file size error:%s", err.Error()))
	}
	data, err = r.ReadSlice('\n') // 文件名
	if err != nil {
		return res, err
	}
	res.Name = strings.TrimSpace(string(data))
	return res, nil
}

func (h *SCP) parseD(r *bufio.Reader) (*D, error) {
	var (
		res = &D{}
	)
	data, err := r.ReadSlice(' ') // 权限位
	if err != nil {
		return res, err
	}
	mode, err := strconv.ParseInt(strings.TrimSpace(string(data)), 8, 64)
	if err != nil {
		return res, errors.New(fmt.Sprintf("parse file mode error:%s", err.Error()))
	}
	res.Mode = os.FileMode(mode)
	data, err = r.ReadSlice(' ') // 大小，忽略
	if err != nil {
		return res, err
	}

	data, err = r.ReadSlice('\n') // 文件名
	if err != nil {
		return res, err
	}
	res.Name = strings.TrimSpace(string(data))
	return res, nil
}

func (h *SCP) parseE(r *bufio.Reader) error {
	_, err := r.ReadSlice('\n') // 消耗掉换行符
	return err
}

func (h *SCP) writeFile(r *bufio.Reader, t *T, c *C, dir string) error {
	// 创建文件
	fpath := path.Join(dir, c.Name)
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, c.Mode)
	if err != nil {
		return err
	}
	defer f.Close()
	// 读取文件内容并写入到文件
	var (
		bufSize  uint64 = 1024
		readSize uint64 = 0
	)
	if c.Size < bufSize {
		bufSize = c.Size
	}
	buff := make([]byte, bufSize)
	for c.Size > 0 { // 只要文件大小不为0，则都要读取数据
		n, err := r.Read(buff)
		if err != nil {
			return err
		}
		readSize += uint64(n)
		_, err = f.Write(buff[:n])
		if err != nil {
			return errors.New(fmt.Sprintf("write to file err:%s", err.Error()))
		}
		if readSize >= c.Size { // 读完了
			break
		}
	}
	// 读取掉 \0
	_, _ = r.ReadSlice(0)
	if t != nil {
		// 修改文件的时间
		err = os.Chtimes(fpath, time.Unix(t.AccessTimestamp, 0), time.Unix(t.ModifyTimestamp, 0))
		if err != nil {
			return errors.New(fmt.Sprintf("change file:%s  time meta err:%s", fpath, err.Error()))
		}
	}
	return nil
}

func (h *SCP) createDirectory(t *T, d *D, dir string) error {
	fpath := path.Join(dir, d.Name)
	err := os.Mkdir(fpath, d.Mode)
	if err != nil {
		return err
	}
	if t != nil {
		err = os.Chtimes(fpath, time.Unix(t.AccessTimestamp, 0), time.Unix(t.ModifyTimestamp, 0))
		if err != nil {
			return errors.New(fmt.Sprintf("change dir:%s  time meta err:%s", fpath, err.Error()))
		}
	}
	return nil
}

func (h *SCP) download(command SCPCommand) {
	fmt.Println("download")
	fpath := command.Destination
	// 检查目标文件
	info, err := os.Stat(fpath)
	if err != nil {
		h.reply(SCPCodeFault, err.Error())
		return
	}
	if info.IsDir() && !command.Recursive {
		h.reply(SCPCodeFault, fmt.Sprintf("%s is a directory", fpath))
	}
	// 读取是否OK
	err = h.readReply()
	if err != nil {
		log.Println("something wrong:" + err.Error())
		return
	}
	//h.replyOK() // 回复ok
	h.dealDownload(h.rw, command.KeepMetaInfo, command.Destination)
}

func (h *SCP) dealDownload(w io.Writer, keepMetaInfo bool, prefixFile string) {
	info, _ := os.Stat(prefixFile)
	if info.IsDir() {
		// 处理文件夹
		err := h.dealDirectory(w, keepMetaInfo, path.Dir(path.Clean(prefixFile)), info)
		if err != nil {
			log.Println(err.Error())
			return
		}
		return
	}
	// 处理文件
	err := h.dealFile(w, keepMetaInfo, path.Dir(path.Clean(prefixFile)), info)
	if err != nil {
		log.Println(err.Error())
		return
	}
}

func (h *SCP) dealFile(w io.Writer, keepMetaInfo bool, prefixDir string, info os.FileInfo) error {
	if keepMetaInfo {
		msg := fmt.Sprintf("T%d 0 %d 0\n", info.ModTime().Unix(), getAccessTime(info).Unix())
		_, err := w.Write([]byte(msg))
		if err != nil {
			return err
		}
		err = h.readReply()
		if err != nil {
			return err
		}
	}
	msg := fmt.Sprintf("C%04o %d %s\n", info.Mode()&os.ModePerm, info.Size(), info.Name())
	_, err := w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = h.readReply()
	if err != nil {
		return err
	}

	// 写入文件内容
	f, err := os.OpenFile(path.Join(prefixDir, info.Name()), os.O_RDONLY, 0666)
	if err != nil {
		h.reply(SCPCodeFault, err.Error())
		return err
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
	_, _ = w.Write([]byte{0}) // 写入结束标志
	err = h.readReply()
	if err != nil { // 读取结果
		return err
	}
	return nil
}

func (h *SCP) dealDirectory(w io.Writer, keepMetaInfo bool, prefixDir string, info os.FileInfo) error {
	if keepMetaInfo {
		msg := fmt.Sprintf("T%d 0 %d 0\n", info.ModTime().Unix(), getAccessTime(info).Unix())
		_, err := w.Write([]byte(msg))
		if err != nil {
			return err
		}
		err = h.readReply()
		if err != nil {
			return err
		}

	}
	msg := fmt.Sprintf("D%04o 0 %s\n", info.Mode()&os.ModePerm, info.Name())
	_, err := w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = h.readReply()
	if err != nil {
		return err
	}

	// 遍历目录
	list, err := os.ReadDir(path.Join(prefixDir, info.Name()))
	if err != nil {
		h.reply(SCPCodeFault, err.Error())
		return err
	}
	for _, item := range list {
		nextPrefixDir := path.Join(prefixDir, info.Name())
		tmpInfo, err := item.Info()
		if err != nil {
			h.reply(SCPCodeFault, "stat "+nextPrefixDir+"/"+item.Name()+" failed:"+err.Error())
			return err
		}
		if item.IsDir() {
			err = h.dealDirectory(w, keepMetaInfo, nextPrefixDir, tmpInfo)
			if err != nil {
				return err
			}
		} else {
			err = h.dealFile(w, keepMetaInfo, nextPrefixDir, tmpInfo)
			if err != nil {
				return err
			}
		}
	}
	//目录遍历完成
	msg = fmt.Sprintf("E\n")
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	return nil
}

func (h *SCP) replyOK() {
	h.reply(SCPCodeOK, "")
}

func (h *SCP) reply(code ScpReplyCode, msg string) {
	switch code {
	case SCPCodeOK:
		h.rw.Write([]byte{0})
	default:
		data := make([]byte, 0)
		data = append(data, code)
		data = append(data, msg[:]...)
		data = append(data, '\n')
		h.rw.Write(data)
	}
}

func (h *SCP) readReply() (err error) {
	var (
		code ScpReplyCode
		r    = bufio.NewReader(h.rw)
	)
	code, err = r.ReadByte()
	if err != nil {
		return err
	}
	if code == SCPCodeOK {
		return nil
	}
	raw, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	msg := strings.TrimSpace(raw)

	if code == SCPCodeFault {
		return errors.New(msg)
	}
	if code == SCPCodeWarning {
		log.Println("warning: " + msg)
	}
	return nil
}

type (
	RW struct {
		r io.Reader
		w io.Writer
	}
)

func (R RW) Write(p []byte) (n int, err error) {
	fmt.Printf("write: %x = %s\n", p, p)
	return R.w.Write(p)
}

func (R RW) Read(p []byte) (n int, err error) {
	n, err = R.r.Read(p)
	fmt.Printf("read: %x = %s\n", p[:n], p[:n])
	return n, err
}
