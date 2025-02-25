package main

import (
	"log"
	"net/http"

	"github.com/gianglt2198/webrtc-tutorial/internal/signaling"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func main() {
	// server := signaling.NewSignalingServer()
	server := signaling.NewDirectSignalingServer()
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Websocket upgrade failed: %v", err)
			return
		}

		server.HandleWebSocket(conn)
	})

	log.Println("Signaling server listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
