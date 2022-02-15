package main

import (
	"io"
	"log"

	"github.com/gliderlabs/ssh"
)

func makePrompt(s ssh.Session) string {
	return s.User() + "@honeypot $ "
}

func sshHandler(s ssh.Session) {
	io.WriteString(s, makePrompt(s))
	io.ReadAll(s)
}

func main() {
	ssh.Handle(sshHandler)
	log.Fatal(ssh.ListenAndServe(":2222", nil))
}
