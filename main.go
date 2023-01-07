package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/senicko/go-ws/ws"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.Upgrade(w, r)
	})

	if err := http.ListenAndServe(":8080", mux); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			fmt.Println("server has been closed")
		} else {
			fmt.Printf("error: %s\n", err)
			os.Exit(1)
		}
	}
}
