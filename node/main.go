package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	// set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// get address info for self and siblings from command line args
	args := os.Args[1:]
	id := args[0]
	servePort := args[1]
	nodes, err := strconv.Atoi(args[2])
	if err != nil {
		logger.Error(fmt.Sprintf("invalid command args: %v\n", err))
		os.Exit(1)
	}
	siblingPorts := args[3 : 3+nodes-1]
	siblings := make(map[string]string, nodes-1)
	for _, sib := range siblingPorts {
		pair := strings.Split(sib, ":")
		siblings[pair[0]] = pair[1]
	}

	balsa := NewBalsa(id, siblings, logger)

	httpPort := args[3+nodes]
	ms := newMapServer(&balsa)
	go ms.start(httpPort)

	balsa.Start(servePort)

	logger.Error(fmt.Sprintf("%s shut down\n", id))
}

func Get(w http.ResponseWriter, r *http.Request) {}
func Set(w http.ResponseWriter, r *http.Request) {}

type mapServer struct {
	balsa *Balsa
	store *Store
}

func newMapServer(balsa *Balsa) mapServer {
	ms := mapServer{
		balsa: balsa,
	}
	http.HandleFunc("/get", ms.get)
	http.HandleFunc("/set", ms.set)
	return ms
}

func (ms *mapServer) get(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	key := params.Get("key")
	if key == "" || r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	val, err := ms.store.Get(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintf(w, "%d", val)
}
func (ms *mapServer) set(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	key := params.Get("key")
	val, err := strconv.Atoi(params.Get("val"))
	if key == "" || err != nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = ms.balsa.AddLog(key, val)
	if err != nil {
		// TODO:
	}
}

func (ms *mapServer) start(port string) {
	http.ListenAndServe(port, nil)
}
