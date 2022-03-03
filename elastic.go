package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
)

var ES_CLIENT *elasticsearch.Client

func setupES() {
	ES_CLIENT, _ = elasticsearch.NewDefaultClient()
}

func sendToESWithCtx(ctx ssh.Context, state *SessionState, doc SubDocument) {
	splat := strings.Split(ctx.RemoteAddr().String(), ":")
	toplevelDoc := SSHDoc{
		Timestamp: time.Now(),
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
		logrus.Errorln("there was an error marshalling an ES document to JSON")
		return
	}
	req := esapi.IndexRequest{
		Index:   "sshdev-index",
		Body:    bytes.NewReader(docBytes),
		Refresh: "true",
	}
	res, err := req.Do(context.Background(), ES_CLIENT)
	if err != nil {
		logrus.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		logrus.Errorf("[%s] Error indexing document\n", res.Status())
	} else {
		// Deserialize the response into a map.
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			logrus.Errorf("Error parsing the response body: %s\n", err)
		} else {
			// Print the response status and indexed document version.
			logrus.Debugf("ES Document index status: [%s] %s; version=%d\n", res.Status(), r["result"], int(r["_version"].(float64)))
		}
	}
}
