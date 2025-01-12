/*

 TODO:
 - figure out browser caching behaviour

*/

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

type nodeInfo struct {
	port string
	sibs []string
}

var nodes = [][]string{
	{
		"35c022aa-129a-4181-a43f-11ee05917262",
		"50051",
		"3",
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08:50052",
		"e46e4d66-595f-4bc3-a503-f7a6068abeec:50053",
	},
	{
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08",
		"50052",
		"3",
		"35c022aa-129a-4181-a43f-11ee05917262:50051",
		"e46e4d66-595f-4bc3-a503-f7a6068abeec:50053",
	},
	{
		"e46e4d66-595f-4bc3-a503-f7a6068abeec",
		"50053",
		"3",

		"35c022aa-129a-4181-a43f-11ee05917262:50051",
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08:50052",
	},
}

func main() {

	fs := http.FileServer(http.Dir("./content"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	http.ListenAndServe(":8080", nil)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error upgrading websocket: %v\n", err)
	}

	// wait for initial start command
	var cmd wsCmd
	err = conn.ReadJSON(&cmd)
	if err != nil {
		log.Printf("error reading message from client: %v\n", err)
		conn.Close()
		return
	}
	if cmd.Cmd != "start" {
		log.Fatalf("invalid initial command: %s\n", cmd.Cmd)
	}

	writeChan := make(chan wsMsg, 100)
	ctx, cancel := context.WithCancel(context.Background())
	sim := sim{
		writer: simWriter{ch: writeChan},
		ctx:    ctx,
		cancel: cancel,
	}
	go sim.start()

	ids := make([]string, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n[0])
	}
	err = conn.WriteJSON(
		wsMsg{
			MsgType: "sim_started",
			Payload: simStartedMsg{
				Ids: ids,
			},
		},
	)

	wsWriter(conn, writeChan)
	sim.cancel()

	log.Printf("closing connection\n")
}

type wsCmd struct {
	Cmd string `json:"cmd"`
}
type wsMsg struct {
	MsgType string      `json:"msg_type"`
	Payload interface{} `json:"payload"`
}
type simStartedMsg struct {
	Ids []string `json:"ids"`
}
type stateUpdateMsg struct {
	Node   string `json:"node"`
	State  string `json:"state"`
	Leader string `json:"leader"`
	Term   int    `json:"term"`
}

func wsWriter(conn *websocket.Conn, writeChan <-chan wsMsg) {
	for msg := range writeChan {
		err := conn.WriteJSON(&msg)
		if err != nil {
			log.Printf("error writing message to client: %v\n", err)
		}
	}
}

type sim struct {
	writer simWriter
	ctx    context.Context
	cancel context.CancelFunc
}
type simWriter struct {
	ch chan<- wsMsg
}

func (sw simWriter) Write(p []byte) (int, error) {
	var logMap map[string]interface{}
	err := json.Unmarshal(p, &logMap)
	if err != nil {
		log.Printf("error unmarshalling log: %v\n", err)
		return len(p), nil
	}

	switch logMap["level"] {
	case "INFO":
		msg := wsMsg{
			MsgType: "state_update",
			Payload: stateUpdateMsg{
				Node:   logMap["node"].(string),
				State:  logMap["state"].(string),
				Leader: logMap["leader"].(string),
				Term:   int(logMap["term"].(float64)),
			},
		}
		sw.ch <- msg
	case "ERROR":
		log.Printf("node error: %s\n", logMap["msg"])
	default:
		log.Printf("invalid level: %v\n", logMap["level"])
	}

	return len(p), nil
}

func (s *sim) start() {
	doneChan := make(chan struct{}, 1)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(nodes))
		for _, args := range nodes {
			go func(ctx context.Context) {
				cmd := exec.CommandContext(ctx, "../node/node", args...)
				cmd.Stdout = s.writer
				cmd.Stderr = s.writer
				err := cmd.Run()
				if err != nil {
					log.Printf("error running node: %v\n", err)
				}
				wg.Done()
			}(s.ctx)
		}
		wg.Wait()
		doneChan <- struct{}{}
	}()

	select {
	case <-s.ctx.Done():
	case <-doneChan:
	}

	log.Printf("sim finished\n")
}
