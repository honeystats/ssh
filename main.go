package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/fatih/color"
	"github.com/gliderlabs/ssh"
	"github.com/honeystats/ssh/files"
	"github.com/sirupsen/logrus"
)

var PORT_NUM string

func envOrFatal(envName string) string {
	val, wasSet := os.LookupEnv(envName)
	if !wasSet {
		logrus.Fatalf("Environment variable missing: $%s", envName)
	}
	return val
}

var DEBUG = false

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		PadLevelText:  true,
	})
	_ = envOrFatal("ELASTICSEARCH_URL")
	PORT_NUM = envOrFatal("PORT")

	_, debugSet := os.LookupEnv("DEBUG")
	if debugSet {
		DEBUG = true
		logrus.SetLevel(logrus.DebugLevel)
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
	Username   string      `json:"username"`
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
	Username string `json:"username"`
}

func (_ DocLogin) action() string {
	return "login"
}

type DocLogout struct {
	Username string `json:"username"`
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

func hostnameOrDefault() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ubuntu"
	}
	return hostname
}

func makePrompt(s ssh.Session, state *SessionState) string {
	hostname := hostnameOrDefault()
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
	if strings.HasPrefix(cmdName, "#") {
		return ""
	}
	switch cmdName {
	case "":
		return ""
	case "clear":
		return "\033c"
	case "ls":
		err, res := ls(state.Root, state.Cwd, args)
		if err != nil {
			return fmt.Sprintf("Error: %s\n", err)
		}
		return res
	case "cd":
		err, res := cd(state.Root, state.Cwd, args)
		if err != nil {
			return fmt.Sprintf("Error: %s\n", err)
		}
		state.Cwd = res
		return ""
	case "cat":
		err, res := cat(state.Root, state.Cwd, args)
		if err != nil {
			return fmt.Sprintf("%s\n", err)
		}
		return res
	case "pwd":
		path := state.Cwd.Path()
		return fmt.Sprintf("%s\n", path)
	case "whoami":
		return fmt.Sprintf("%s\n", ctx.User())
	default:
		return fmt.Sprintf("command not found: %s\n", cmd)
	}
}

var commandList = []string{
	"cat",
	"cd",
	"clear",
	"exit",
	"ls",
	"pwd",
	"whoami",
}

// Returns string to print and whether to repopulate (if there were conflicts)
func tabCompleteFile(state *SessionState, partialFile string) (string, bool) {
	startDir := state.Cwd
	searchFile := partialFile
	if strings.HasPrefix(partialFile, "/") {
		startDir = FILESYSTEM.Root
		searchFile = strings.TrimPrefix(partialFile, "/")
	}
	lastSlash := strings.LastIndex(partialFile, "/")
	if lastSlash > 0 {
		dirPath := partialFile[0 : lastSlash+1]
		err, res := startDir.GetFileOrDir(state.Root, dirPath)
		if err != nil {
			return "", false
		}
		startDir = res.(*files.FilesystemDir)
		searchFile = partialFile[lastSlash+1:]
	}
	validFileDir := []files.FileDir{}
	for _, file := range startDir.Files {
		validFileDir = append(validFileDir, file)
	}
	for _, dir := range startDir.Subdirs {
		validFileDir = append(validFileDir, dir)
	}
	one := false
	multiple := false
	last := ""
	allValid := []string{}
	for _, validFileDir := range validFileDir {
		if strings.HasPrefix(validFileDir.TabcompleteName(), searchFile) {
			if one == true {
				multiple = true
			}
			one = true
			last = strings.TrimPrefix(validFileDir.TabcompleteName(), searchFile)
			allValid = append(allValid, validFileDir.DescribeSelf())
		}
	}
	if one && !multiple {
		return last, false
	}
	if multiple {
		return strings.Join(allValid, " "), true
	}
	return "", false
}

func tabCompleteCmd(state *SessionState, partialCmd string) (string, bool) {
	one := false
	multiple := false
	last := ""
	allValid := []string{}
	for _, validCommand := range commandList {
		if strings.HasPrefix(validCommand, partialCmd) {
			if one == true {
				multiple = true
			}
			one = true
			last = strings.TrimPrefix(validCommand, partialCmd)
			allValid = append(allValid, validCommand)
		}
	}
	if one && !multiple {
		return last + " ", false
	}
	if multiple {
		return strings.Join(allValid, " "), true
	}
	return "", false
}

func tabComplete(state *SessionState, cmd string) (string, bool) {
	parts := strings.Split(cmd, " ")
	numParts := len(parts)
	if numParts == 1 {
		return tabCompleteCmd(state, parts[0])
	} else {
		return tabCompleteFile(state, parts[numParts-1])
	}
}

func sshHandler(s ssh.Session) {
	ctx := s.Context().(ssh.Context)
	sessionId := ctx.SessionID()
	state := sessionMap.getOrCreateById(sessionId)
	logrus.WithFields(logrus.Fields{
		"user": s.User(),
		"id":   sessionId,
	}).Infoln("SSH session opened")
	sendToES := func(doc SubDocument) {
		go sendToESWithCtx(ctx, state, doc)
	}
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s, state))
	sendToES(DocLogin{
		Username: s.User(),
	})
	var cmd []byte = []byte{}
	for {
		oneByte, err := reader.ReadByte()
		logrus.Debugf("%#v\n", oneByte)
		if err != nil {
			s.Close()
			return
		}
		doLogout := func() {
			logrus.WithFields(logrus.Fields{
				"user": s.User(),
				"id":   sessionId,
			}).Infoln("SSH session closed")
			sendToES(DocLogout{
				Username: s.User(),
			})
			s.Close()
		}
		switch oneByte {
		case '\x04': // Ctrl+D / EOF
			if len(cmd) != 0 {
				continue
			}
			io.WriteString(s, "logout\n")
			doLogout()
			return
		case '\x0c': // Ctrl+L
			res := runCmd(ctx, state, "clear")
			io.WriteString(s, res)
			io.WriteString(s, makePrompt(s, state))
		case '\x0d': // Return
			sendToES(DocCommandRun{
				Command: string(cmd),
			})
			io.WriteString(s, "\n")
			res := runCmd(ctx, state, string(cmd))
			if string(cmd) == "exit" || strings.HasPrefix(string(cmd), "exit ") {
				doLogout()
				return
			}
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
		case '\t':
			res, repop := tabComplete(state, string(cmd))
			if repop {
				io.WriteString(s, "\n"+res+"\n"+makePrompt(s, state)+string(cmd))
			} else {
				cmd = append(cmd, res...)
				io.WriteString(s, res)
			}
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
	Root      *files.FilesystemDir `json:"-"`
	Cwd       *files.FilesystemDir `json:"cwd"`
	Passwords []string             `json:"passwords"`
	Keys      []SSHKey             `json:"keys"`
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
		Root:      FILESYSTEM.Root,
		Cwd:       FILESYSTEM.Root,
		Passwords: []string{},
		Keys:      []SSHKey{},
	}
	m[id] = &newState
	return &newState
}

func main() {
	setupES()
	hostname := hostnameOrDefault()
	key, err := genHostKey(hostname)
	if err != nil {
		logrus.WithError(err).Fatal("Error generating private key")
	}
	hostKeySigner, err := gossh.NewSignerFromKey(key)

	if err != nil {
		logrus.WithError(err).Fatal("Error generating host key signer")
	}
	srv := &ssh.Server{
		Addr:             ":" + PORT_NUM,
		Handler:          sshHandler,
		PublicKeyHandler: pubKeyHandler,
		PasswordHandler:  passwordHandler,
		HostSigners: []ssh.Signer{
			hostKeySigner,
		},
		Version: "OpenSSH_8.4p1 Ubuntu-6ubuntu2.1",
	}
	logrus.Infoln("Waiting for SSH connections...")
	logrus.Fatalln(srv.ListenAndServe())
}
