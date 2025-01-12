/*

 TODO:
 - figure out browser caching behaviour

*/

package main

import (
	"bytes"
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

	readChan := make(chan wsCmd)
	writeChan := make(chan wsMsg)

	ctx, cancel := context.WithCancel(context.Background())
	sim := sim{
		writer:  simWriter{ch: writeChan},
		ctx:     ctx,
		cancel:  cancel,
		running: false,
	}

	go wsWriter(conn, writeChan)
	go wsReader(conn, readChan)

	ids := []string{}
	for _, n := range nodes {
		ids = append(ids, n[0])
	}
	for cmd := range readChan {
		switch cmd.Cmd {
		case "start":
			if !sim.running {
				go sim.start()
			}
			writeChan <- wsMsg{
				MsgType: "sim_started",
				Payload: simStartedMsg{
					Ids: ids,
				},
			}
		default:
			log.Printf("invalid command: %s\n", cmd.Cmd)
		}
	}

	sim.cancel()
	close(writeChan)

	log.Printf("sim ended\n")
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
func wsReader(conn *websocket.Conn, readChan chan<- wsCmd) {
	for {
		var cmd wsCmd
		err := conn.ReadJSON(&cmd)
		if err != nil {
			if websocket.IsCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
			) {
				close(readChan)
				return
			}
			log.Printf("error reading message: %v\n", err)
		}
		readChan <- cmd
	}
}

type sim struct {
	writer  simWriter
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}
type simWriter struct {
	ch chan<- wsMsg
}

func (sw simWriter) Write(p []byte) (int, error) {
	logs := bytes.Split(p, []byte{'\n'})
	logMaps := make([]map[string]interface{}, len(logs))
	for i, l := range logs {
		if len(l) == 0 {
			continue
		}
		err := json.Unmarshal(l, &logMaps[i])
		if err != nil {
			log.Printf("error unmarshalling log: %v\n", err)
			return len(p), nil
		}

		switch logMaps[i]["level"] {
		case "INFO":
			msg := wsMsg{
				MsgType: "state_update",
				Payload: stateUpdateMsg{
					Node:   logMaps[i]["node"].(string),
					State:  logMaps[i]["state"].(string),
					Leader: logMaps[i]["leader"].(string),
					Term:   int(logMaps[i]["term"].(float64)),
				},
			}
			sw.ch <- msg
		case "ERROR":
			log.Printf("node error: %s\n", logMaps[i]["msg"])
		default:
			log.Printf("invalid level: %v\n", logMaps[i]["level"])
		}
	}

	return len(p), nil
}

func (s *sim) start() {
	s.running = true
	doneChan := make(chan struct{}, 1)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(nodes))
		for _, args := range nodes {
			go func(ctx context.Context) {
				cmd := exec.CommandContext(ctx, "../node/node", args...)
				cmd.Stdout = s.writer
				cmd.Stderr = s.writer
				cmd.Run()
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

	s.running = false

	log.Printf("sim finished\n")
}
