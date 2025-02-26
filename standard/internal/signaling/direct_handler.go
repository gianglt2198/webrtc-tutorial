package signaling

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gianglt2198/webrtc-tutorial/internal/models"
	"github.com/gianglt2198/webrtc-tutorial/internal/peer"
	"github.com/gianglt2198/webrtc-tutorial/internal/room"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// Message Types
const (
	TypeJoin             = "join"
	TypeJoined           = "joined"
	TypeOffer            = "offer"
	TypeAnswer           = "answer"
	TypeCandidate        = "candidate"
	TypeNewParticipant   = "new-participant"
	TypeGetParticipants  = "get-participants"
	TypeParticipantLeft  = "participant-left"
	TypeParticipantsList = "participants-list"
	TypeLeave            = "leave"
)

type DirectSignalingServer struct {
	Rooms map[string]*room.Room
	Lock  sync.RWMutex
}

func NewDirectSignalingServer() *DirectSignalingServer {
	return &DirectSignalingServer{
		Rooms: make(map[string]*room.Room),
	}
}

func (s *DirectSignalingServer) HandleWebSocket(conn *websocket.Conn) {
	defer conn.Close()

	var currentRoom *room.Room
	peerID := ""

	defer func() {
		if currentRoom != nil && peerID != "" {
			s.handleLeave(peerID, currentRoom)
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		var signalMsg models.DirectSignalMessage
		if err := json.Unmarshal(msg, &signalMsg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		switch signalMsg.Type {
		case TypeJoin:
			peerID = signalMsg.SenderID
			s.handleJoin(conn, peerID, &signalMsg, &currentRoom)
		case TypeOffer, TypeAnswer, TypeCandidate:
			s.routeMessage(peerID, &signalMsg, currentRoom)
		case TypeGetParticipants:
			s.handleGetParticipants(peerID, currentRoom)
		case TypeLeave:
			s.handleLeave(peerID, currentRoom)
			return
		default:
			log.Printf("Unknown message type: %s", signalMsg.Type)
		}
	}
}

func (s *DirectSignalingServer) handleJoin(conn *websocket.Conn, peerID string, msg *models.DirectSignalMessage, currentRoom **room.Room) {
	s.Lock.Lock()
	defer s.Lock.Unlock()

	roomID := msg.RoomID
	if roomID == "" {
		roomID = "default-room"
	}

	r, exists := s.Rooms[roomID]
	if !exists {
		r = &room.Room{
			ID:        roomID,
			Peers:     make(map[string]*peer.Peer),
			CreatedAt: time.Now(),
		}
		s.Rooms[roomID] = r
	}

	pc, err := peer.NewPeerConnection(conn)
	if err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	r.Lock.Lock()
	r.Peers[peerID] = pc
	r.Lock.Unlock()

	*currentRoom = r

	// Send joined confirmation
	response := models.DirectSignalMessage{
		Type:     TypeJoined,
		RoomID:   roomID,
		SenderID: peerID,
	}
	s.sendMessage(pc, response)
	// Notify others about new participant
	s.notifyNewParticipant(peerID, r)
	// Send existing participants to new peer
	s.sendExistingParticipants(peerID, r)

	// Setup ICE candidate handling
	pc.PeerConn.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		candidateData, err := json.Marshal(c.ToJSON())
		if err != nil {
			log.Printf("Error marshaling ICE candidate: %v", err)
			return
		}

		msg := models.DirectSignalMessage{
			Type:     TypeCandidate,
			RoomID:   roomID,
			SenderID: peerID,
			Payload:  candidateData,
		}
		s.sendMessage(pc, msg)
	})

	pc.PeerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("peer.Peer %s connection state: %s", peerID, state.String())
		if state == webrtc.PeerConnectionStateDisconnected {
			s.handleLeave(peerID, r)
		}
	})
}

func (s *DirectSignalingServer) routeMessage(senderID string, msg *models.DirectSignalMessage, room *room.Room) {
	room.Lock.RLock()
	defer room.Lock.RUnlock()

	senderPeer, exists := room.Peers[senderID]
	if !exists {
		log.Printf("Sender %s not found in room %s", senderID, room.ID)
		return
	}

	if msg.TargetID != "" {
		// Direct message to specific peer
		if targetPeer, exists := room.Peers[msg.TargetID]; exists {
			s.sendMessage(targetPeer, *msg)
		}
		return
	}

	switch msg.Type {
	case TypeOffer, TypeAnswer:
		var rawJson string
		_ = json.Unmarshal([]byte(msg.Payload), &rawJson)

		var sd webrtc.SessionDescription
		if err := json.Unmarshal([]byte(rawJson), &sd); err != nil {
			log.Printf("Error Unmarshaling SDP: %v", err)
			return
		}

		switch msg.Type {
		case TypeOffer:
			if err := senderPeer.PeerConn.SetRemoteDescription(sd); err != nil {
				log.Printf("Error setting remote description: %v", err)
				return
			}

			answer, err := senderPeer.PeerConn.CreateAnswer(nil)
			if err != nil {
				log.Printf("Error creating answer: %v", err)
				return
			}

			if err = senderPeer.PeerConn.SetLocalDescription(answer); err != nil {
				log.Printf("Error setting local description: %v", err)
				return
			}

			answerData, err := json.Marshal(answer)
			if err != nil {
				log.Printf("Error marshaling answer: %v", err)
				return
			}

			response := models.DirectSignalMessage{
				Type:     TypeAnswer,
				RoomID:   room.ID,
				SenderID: senderID,
				Payload:  answerData,
			}
			s.sendMessage(senderPeer, response)

		case TypeAnswer:
			if err := senderPeer.PeerConn.SetRemoteDescription(sd); err != nil {
				log.Printf("Error setting remote answer: %v", err)
			}
		}

	case TypeCandidate:
		var rawJson string
		_ = json.Unmarshal([]byte(msg.Payload), &rawJson)

		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal([]byte(rawJson), &candidate); err != nil {
			log.Printf("Error unmarshaling candidate: %v", err)
			return
		}

		if err := senderPeer.PeerConn.AddICECandidate(candidate); err != nil {
			log.Printf("Error adding ICE candidate: %v", err)
		}
	}
}

func (s *DirectSignalingServer) handleLeave(peerID string, room *room.Room) {
	room.Lock.Lock()
	defer room.Lock.Unlock()

	fmt.Println("handleLeave", peerID)

	if peer, exists := room.Peers[peerID]; exists {
		if peer.PeerConn != nil {
			peer.PeerConn.Close()
		}
		delete(room.Peers, peerID)
	}

	// Notify other participants
	leaveMsg := models.DirectSignalMessage{
		Type:     TypeParticipantLeft,
		RoomID:   room.ID,
		SenderID: peerID,
	}
	s.broadcastMessage(room, leaveMsg, peerID)

	if len(room.Peers) == 0 {
		s.Lock.Lock()
		delete(s.Rooms, room.ID)
		s.Lock.Unlock()
	}
}

func (s *DirectSignalingServer) handleGetParticipants(peerID string, room *room.Room) {
	room.Lock.RLock()
	defer room.Lock.RUnlock()

	peersList := make([]string, 0, len(room.Peers))
	for id := range room.Peers {
		if id != peerID {
			peersList = append(peersList, id)
		}
	}

	response := models.DirectSignalMessage{
		Type:     TypeParticipantsList,
		RoomID:   room.ID,
		SenderID: peerID,
		Peers:    peersList,
	}

	if peer, exists := room.Peers[peerID]; exists {
		s.sendMessage(peer, response)
	}
}

func (s *DirectSignalingServer) notifyNewParticipant(newPeerID string, room *room.Room) {
	msg := models.DirectSignalMessage{
		Type:     TypeNewParticipant,
		RoomID:   room.ID,
		SenderID: newPeerID,
	}
	s.broadcastMessage(room, msg, newPeerID)
}

func (s *DirectSignalingServer) sendExistingParticipants(newPeerID string, room *room.Room) {
	room.Lock.RLock()
	defer room.Lock.RUnlock()

	peersList := make([]string, 0, len(room.Peers)-1)
	for id := range room.Peers {
		if id != newPeerID {
			peersList = append(peersList, id)
		}
	}

	if len(peersList) > 0 {
		response := models.DirectSignalMessage{
			Type:     TypeParticipantsList,
			RoomID:   room.ID,
			SenderID: newPeerID,
			Peers:    peersList,
		}
		if peer, exists := room.Peers[newPeerID]; exists {
			s.sendMessage(peer, response)
		}
	}
}

func (s *DirectSignalingServer) broadcastMessage(room *room.Room, msg models.DirectSignalMessage, excludePeer string) {
	for id, peer := range room.Peers {
		if id != excludePeer {
			s.sendMessage(peer, msg)
		}
	}
}

func (s *DirectSignalingServer) sendMessage(peer *peer.Peer, msg models.DirectSignalMessage) {
	if peer == nil || peer.Conn == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := peer.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
