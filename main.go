package main

import (
	"bytes"
	"flag"
	"fmt"
	sshd "github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh"
	"net"
	"time"
)

func main() {
	var (
		port     uint = 4400
		username      = "tools"
		password      = "tools"
	)

	flag.UintVar(&port, "port", 4400, "the listen port")
	flag.StringVar(&username, "username", "tools", "the ssh auth username")
	flag.StringVar(&password, "password", "tools", "the ssh auth password")
	flag.Parse()

	fmt.Printf("listen at *:%d\n", port)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	signer, _ := ssh.ParsePrivateKey([]byte(PRIKEY))
	srv := &sshd.Server{
		Addr:                       "",
		Handler:                    (&SSH{}).Handle,
		HostSigners:                []sshd.Signer{signer},
		Version:                    "toolkits",
		KeyboardInteractiveHandler: nil,
		PasswordHandler: func(ctx sshd.Context, password string) bool {
			if ctx.User() == username && password == password {
				return true
			}
			return false
		},
		PublicKeyHandler: func(ctx sshd.Context, key sshd.PublicKey) bool {
			pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(PUBKEY))
			if err != nil {
				panic(err)
			}
			if bytes.Equal(key.Marshal(), pk.Marshal()) {
				return true
			}
			return false
		},
		LocalPortForwardingCallback:   nil,
		ReversePortForwardingCallback: nil,
		SessionRequestCallback:        nil,
		ConnectionFailedCallback:      nil,
		IdleTimeout:                   time.Hour * 12,
		MaxTimeout:                    time.Hour * 12,
		ChannelHandlers:               nil,
		RequestHandlers:               nil,
		SubsystemHandlers:             nil,
	}
	initSubsystemHandler(srv)

	err = srv.Serve(l)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}

func initSubsystemHandler(srv *sshd.Server) {
	srv.SubsystemHandlers = map[string]sshd.SubsystemHandler{
		"sftp": NewSFTPServer().Handle,
	}
}
