/*

 TODO:
 - figure out browser caching behaviour

*/

package main

import (
	"context"
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
	go wsReader(conn, readChan)

	writeChan := make(chan wsMsg)
	go wsWriter(conn, writeChan)

	var sim sim
	sim.writer = simWriter{ch: writeChan}
	for msg := range readChan {
		switch msg.Cmd {
		case "start":
			if !sim.started {
				sim.started = true
				ctx, cancel := context.WithCancel(context.Background())
				sim.ctx = ctx
				sim.cancel = cancel
				go sim.start()
			}
		default:
			log.Printf("invalid message from client: %v\n", msg)
		}
	}
	// TODO: stop sim
	if sim.started {
		sim.cancel()
	}
	close(writeChan)
	log.Printf("closing connection\n")
}

type wsCmd struct {
	Cmd string `json:"cmd"`
}
type wsMsg struct {
	MsgType string      `json:"msg_type"`
	Payload interface{} `json:"payload"`
}
type logMsg struct {
	Log string `json:"log"`
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
				log.Printf("client closed connection\n")
				close(readChan)
				return
			}
			log.Printf("error reading message from client: %v\n", err)
		}

		readChan <- cmd
	}
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
	started bool
	writer  simWriter
	ctx     context.Context
	cancel  context.CancelFunc
}
type simWriter struct {
	ch chan<- wsMsg
}

func (sw simWriter) Write(p []byte) (int, error) {
	log := string(p)
	sw.ch <- wsMsg{MsgType: "log", Payload: logMsg{Log: log}}
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

	s.started = false
	log.Printf("sim finished\n")
}
