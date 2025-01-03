package main

import (
	"os"
	"os/exec"
)

// import "net/http"

func main() {
	// http.Handle("/", http.FileServer(http.Dir("./content")))
	// http.ListenAndServe(":3000", nil)
	node1 := exec.Command("../node/node", "50051 2 50052")
	node2 := exec.Command("../node/node", "50052 2 50051")
	node1.Stdout = os.Stdout
	node2.Stdout = os.Stdout
	go node1.Run()
	go node2.Run()
	for {
	}
}
