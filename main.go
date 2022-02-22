package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	gossh "golang.org/x/crypto/ssh"

	"github.com/fatih/color"
	"github.com/gliderlabs/ssh"
)

var PORT_NUM string

func init() {
	_, urlSet := os.LookupEnv("ELASTICSEARCH_URL")
	if !urlSet {
		panic("ELASTICSEARCH_URL is not set.")
	}
	var portSet bool
	PORT_NUM, portSet = os.LookupEnv("PORT")
	if !portSet {
		panic("PORT is not set.")
	}
}

type SSHDoc struct {
	Action     string      `json:"action"`
	SourceIP   string      `json:"sourceIP"`
	SourcePort string      `json:"sourcePort"`
	Fields     SubDocument `json:"fields"`
}

type SubDocument interface {
	action() string
}

type DocCommandRun struct {
	Command string `json:"command"`
	User    string `json:"user"`
}

func (_ DocCommandRun) action() string {
	return "command_run"
}

type DocLogin struct {
	User string `json:"user"`
}

func (_ DocLogin) action() string {
	return "login"
}

type DocLogout struct {
	User string `json:"user"`
}

func (_ DocLogout) action() string {
	return "logout"
}

type DocPubkey struct {
	Key string `json:"key"`
}

func (_ DocPubkey) action() string {
	return "tried_pubkey"
}

type DocPassword struct {
	Password string `json:"password"`
}

func (_ DocPassword) action() string {
	return "tried_password"
}

func makePrompt(s ssh.Session) string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ubuntu"
	}
	userAtHost := color.HiGreenString(s.User() + "@" + hostname)
	path := color.HiBlueString("~")
	promptStr := color.WhiteString("$ ")
	return userAtHost + ":" + path + promptStr
}

func runCmd(cmd string) string {
	splat := strings.Split(cmd, " ")
	if len(splat) < 1 {
		return ""
	}
	cmdName := splat[0]
	args := cmd[len(cmdName):]
	switch cmdName {
	case "ls":
		err, res := ls(args)
		if err != nil {
			return fmt.Sprintf("Error: %s\n", err)
		}
		return res
	default:
		return fmt.Sprintf("No such command found: %s\n", cmd)
	}
}

func sshHandler(s ssh.Session) {
	sendToES := func(doc SubDocument) {
		sendToESWithCtx(s.RemoteAddr(), doc)
	}
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s))
	sendToES(DocLogin{
		User: s.User(),
	})
	var cmd []byte = []byte{}
	for {
		oneByte, err := reader.ReadByte()
		fmt.Printf("%#v\n", oneByte)
		if err != nil {
			s.Close()
			return
		}
		switch oneByte {
		case '\x04': // Ctrl+D / EOF
			if len(cmd) != 0 {
				continue
			}
			sendToES(DocLogout{
				User: s.User(),
			})
			io.WriteString(s, "logout\n")
			s.Close()
			return
		case '\x0d': // Return
			sendToES(DocCommandRun{
				Command: string(cmd),
				User:    s.User(),
			})
			io.WriteString(s, "\n")
			res := runCmd(string(cmd))
			cmd = []byte{}
			io.WriteString(s, res)
			io.WriteString(s, makePrompt(s))
		case '\x7f': // Backspace
			if len(cmd) < 1 {
				cmd = []byte{}
				continue
			}
			cmd = cmd[0 : len(cmd)-1]
			io.WriteString(s, "\x08 \x08")
		case '\x1b': // Arrow Key Escape 1
		case '\x5b': // Arrow Key Escape 2
		case '\x41': // Arrow Key
		case '\x42': // Arrow Key
		case '\x43': // Arrow Key
		case '\x44': // Arrow Key
		case '\x03': // Ctrl+C
			cmd = append(cmd, '^', 'C')
			sendToES(DocCommandRun{
				Command: string(cmd),
				User:    s.User(),
			})
			io.WriteString(s, "^C\n")
			io.WriteString(s, makePrompt(s))
			cmd = []byte{}
		default:
			cmd = append(cmd, oneByte)
			io.WriteString(s, string(oneByte))
		}
	}
}

func pubKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	sendToESWithCtx(ctx.RemoteAddr(), DocPubkey{
		Key: string(gossh.MarshalAuthorizedKey(key)),
	})
	return false
}

func passwordHandler(ctx ssh.Context, password string) bool {
	sendToESWithCtx(ctx.RemoteAddr(), DocPassword{
		Password: password,
	})
	// return password == "ubuntu"
	return true
}

func main() {
	setupES()
	srv := &ssh.Server{
		Addr:             ":" + PORT_NUM,
		Handler:          sshHandler,
		PublicKeyHandler: pubKeyHandler,
		PasswordHandler:  passwordHandler,
	}
	srv.Version = "OpenSSH_8.4p1 Ubuntu-6ubuntu2.1"
	log.Fatal(srv.ListenAndServe())
}
