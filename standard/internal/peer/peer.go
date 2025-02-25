package peer

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gianglt2198/webrtc-tutorial/internal/models"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Peer struct {
	Conn      *websocket.Conn
	PeerConn  *webrtc.PeerConnection
	SendMutex sync.Mutex
}

func (p *Peer) SendMessage(msg models.SignalMessage) {
	p.SendMutex.Lock()
	defer p.SendMutex.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
