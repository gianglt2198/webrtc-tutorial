package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

const (
	webPort = ":8081"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	fmt.Printf("Starting server at localhost%v\n", webPort)
	log.Fatal(http.ListenAndServe(webPort, nil))

}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	defer conn.Close()

	// Handle incoming message
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		// Process signaling messages
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println(err)
			continue
		}

		switch msg["type"] {
		case "join":
		case "offer":
		case "answer":
		case "candidate":
		}
	}
}

func createPeerConnection() (*webrtc.PeerConnection, error) {
	// Define ICE servers
	iceServers := []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	}

	// Create a new RTCPeerConnection
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Handle ICE connection state changes
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", state.String())
	})

	return peerConnection, nil
}
