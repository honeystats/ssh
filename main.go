package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

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
	Cwd        string      `json:"cwd"`
	Passwords  []string    `json:"passwords"`
	Keys       []SSHKey    `json:"keys"`
	Fields     SubDocument `json:"fields"`
	SessionID  string      `json:"sessionId"`
	User       string      `json:"user"`
	Timestamp  time.Time   `json:"@timestamp"`
}

type SubDocument interface {
	action() string
}

type DocCommandRun struct {
	Command string `json:"command"`
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

func makePrompt(s ssh.Session, state *SessionState) string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ubuntu"
	}
	userAtHost := color.HiGreenString(s.User() + "@" + hostname)
	path := color.HiBlueString(state.Cwd.Path())
	promptStr := color.WhiteString("$ ")
	return userAtHost + ":" + path + promptStr
}

func runCmd(ctx ssh.Context, state *SessionState, cmd string) string {
	splat := strings.Split(cmd, " ")
	if len(splat) < 1 {
		return ""
	}
	cmdName := splat[0]
	args := cmd[len(cmdName):]
	if len(args) > 0 && args[0] == ' ' {
		args = args[1:]
	}
	switch cmdName {
	case "ls":
		err, res := ls(state.Cwd, args)
		if err != nil {
			return fmt.Sprintf("Error: %s\n", err)
		}
		return res
	case "cd":
		err, res := cd(state.Cwd, args)
		if err != nil {
			return fmt.Sprintf("Error: %s\n", err)
		}
		state.Cwd = res
		return ""
	case "pwd":
		path := state.Cwd.Path()
		return fmt.Sprintf("%s\n", path)
	case "whoami":
		return fmt.Sprintf("%s\n", ctx.User())
	case "#":
		return ""
	default:
		return fmt.Sprintf("No such command found: %s\n", cmd)
	}
}

func sshHandler(s ssh.Session) {
	ctx := s.Context().(ssh.Context)
	sessionId := ctx.SessionID()
	state := sessionMap.getOrCreateById(sessionId)
	sendToES := func(doc SubDocument) {
		sendToESWithCtx(ctx, state, doc)
	}
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s, state))
	sendToES(DocLogin{
		User: s.User(),
	})
	var cmd []byte = []byte{}
	for {
		oneByte, err := reader.ReadByte()
		// fmt.Printf("%#v\n", oneByte)
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
			})
			io.WriteString(s, "\n")
			res := runCmd(ctx, state, string(cmd))
			cmd = []byte{}
			io.WriteString(s, res)
			io.WriteString(s, makePrompt(s, state))
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
			})
			io.WriteString(s, "^C\n")
			io.WriteString(s, makePrompt(s, state))
			cmd = []byte{}
		default:
			cmd = append(cmd, oneByte)
			io.WriteString(s, string(oneByte))
		}
	}
}

func pubKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	strKey := string(gossh.MarshalAuthorizedKey(key))

	sessionId := ctx.SessionID()
	curState := sessionMap.getOrCreateById(sessionId)
	curState.Keys = append(curState.Keys, SSHKey{
		Key:  strKey,
		Type: key.Type(),
	})
	sendToESWithCtx(ctx, curState, DocPubkey{
		Key: strKey,
	})
	return false
}

func passwordHandler(ctx ssh.Context, password string) bool {
	sessionId := ctx.SessionID()
	curState := sessionMap.getOrCreateById(sessionId)
	curState.Passwords = append(curState.Passwords, password)
	sendToESWithCtx(ctx, curState, DocPassword{
		Password: password,
	})
	// return password == "ubuntu"
	return true
}

type SSHKey struct {
	Key  string `json:"key"`
	Type string `json:"type"`
}

type SessionState struct {
	Cwd       *FilesystemDir `json:"cwd"`
	Passwords []string       `json:"passwords"`
	Keys      []SSHKey       `json:"keys"`
}

// Map from session ID to session state
type SessionMap map[string]*SessionState

var sessionMap SessionMap = SessionMap{}

func (m SessionMap) getOrCreateById(id string) *SessionState {
	state, exists := m[id]
	if exists {
		return state
	}
	var newState SessionState = SessionState{
		Cwd:       FILESYSTEM.Root,
		Passwords: []string{},
		Keys:      []SSHKey{},
	}
	m[id] = &newState
	return &newState
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
