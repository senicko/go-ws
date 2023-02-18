package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/senicko/go-ws/ws"
)

func main() {
	mux := http.NewServeMux()

	upgrader := ws.Upgrader{
		Compress: true,
	}

	clis := []*ws.Conn{}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		defer conn.Close()
		if err != nil {
			log.Println(err)
			return
		}

		clis = append(clis, conn)

		for {
			message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read failed:", err)
				break
			}

			h, m, s := time.Now().Clock()
			message = append([]byte(fmt.Sprintf("%d:%d:%d -> ", h, m, s)), message...)
			fmt.Println(string(message))

			for _, cli := range clis {
				if err := cli.WriteMessage(ws.OpText, message); err != nil {
					fmt.Println("failed to send the message:", err)
				}
			}
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
