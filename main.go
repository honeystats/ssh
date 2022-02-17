package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
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
	Action string      `json:"action"`
	Fields SubDocument `json:"fields"`
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
	User   string `json:"user"`
	PubKey string `json:"pubKey"`
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

func sshHandler(s ssh.Session) {
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s))
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
			cmd = []byte{}
			io.WriteString(s, "\n")
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

func sendToES(doc SubDocument) {
	toplevelDoc := SSHDoc{
		Action: doc.action(),
		Fields: doc,
	}
	docBytes, err := json.Marshal(toplevelDoc)
	if err != nil {
		fmt.Println("there was an error marshalling the document to JSON")
		return
	}
	req := esapi.IndexRequest{
		Index:   "sshdev-index",
		Body:    bytes.NewReader(docBytes),
		Refresh: "true",
	}
	res, err := req.Do(context.Background(), ES_CLIENT)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("[%s] Error indexing document", res.Status())
	} else {
		// Deserialize the response into a map.
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			log.Printf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and indexed document version.
			log.Printf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
		}
	}
}

var ES_CLIENT *elasticsearch.Client

func setupES() {
	ES_CLIENT, _ = elasticsearch.NewDefaultClient()
	log.Println(ES_CLIENT.Info())
}

func main() {
	setupES()
	srv := &ssh.Server{Addr: ":" + PORT_NUM, Handler: sshHandler}
	srv.Version = "OpenSSH_8.4p1 Ubuntu-6ubuntu2.1"
	log.Fatal(srv.ListenAndServe())
}
