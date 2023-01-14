package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/senicko/go-ws/ws"
)

func main() {
	mux := http.NewServeMux()

	upgrader := ws.Upgrader{}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		defer conn.Close()

		if err != nil {
			log.Println(err)
			return
		}

		for {
			message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read failed:", err)
				break
			}

			conn.SendMessage(ws.TextFrame, message)
		}
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
