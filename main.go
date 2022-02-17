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

type ESDocument struct {
	Action  string `json:"action"`
	Command string `json:"command"`
	User    string `json:"user"`
}

func makePrompt(s ssh.Session) string {
	return s.User() + "@honeypot $ "
}

func sshHandler(s ssh.Session) {
	reader := bufio.NewReader(s)
	io.WriteString(s, makePrompt(s))
	var cmd []byte = []byte{}
	for {
		oneByte, err := reader.ReadByte()
		cmd = append(cmd, oneByte)
		fmt.Printf("%#v\n", oneByte)
		if err != nil {
			cmd = []byte{}
			return
		}
		if oneByte == '\x04' {
			cmd = []byte{}
			s.Close()
			return
		}
		io.WriteString(s, string(oneByte))
		if oneByte == '\x0d' {
			sendToES(ESDocument{
				Action:  "command_run",
				Command: string(cmd),
				User:    s.User(),
			})
			cmd = []byte{}
			io.WriteString(s, string('\n'))
			io.WriteString(s, makePrompt(s))
		}
	}
}

func sendToES(doc ESDocument) {
	docBytes, err := json.Marshal(doc)
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
