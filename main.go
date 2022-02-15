package main

import (
	"bufio"
	"fmt"
	"io"
	"log"

	"github.com/gliderlabs/ssh"
)

func makePrompt(s ssh.Session) string {
	return s.User() + "@honeypot $ "
}

func sshHandler(s ssh.Session) {
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s))
	for {
		oneByte, err := reader.ReadByte()
		fmt.Printf("%#v\n", oneByte)
		if err != nil {
			return
		}
		if oneByte == '\x04' {
			s.Close()
			return
		}
		io.WriteString(s, string(oneByte))
		if oneByte == '\x0d' {
			io.WriteString(s, string('\n'))
			io.WriteString(s, makePrompt(s))
		}
	}
}

func main() {
	srv := &ssh.Server{Addr: ":2222", Handler: sshHandler}
	srv.Version = "OpenSSH_8.4p1 Ubuntu-6ubuntu2.1"
	log.Fatal(srv.ListenAndServe())
}
