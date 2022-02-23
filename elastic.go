package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/gliderlabs/ssh"
)

var ES_CLIENT *elasticsearch.Client

func setupES() {
	ES_CLIENT, _ = elasticsearch.NewDefaultClient()
}

func sendToESWithCtx(ctx ssh.Context, state *SessionState, doc SubDocument) {
	splat := strings.Split(ctx.RemoteAddr().String(), ":")
	toplevelDoc := SSHDoc{
		Action:    doc.action(),
		Cwd:       state.Cwd.Path(),
		Passwords: state.Passwords,
		Keys:      state.Keys,
		Fields:    doc,
		SessionID: ctx.SessionID(),
		User:      ctx.User(),
	}
	if len(splat) == 2 {
		toplevelDoc.SourceIP = splat[0]
		toplevelDoc.SourcePort = splat[1]
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
			// log.Printf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
		}
	}
}
