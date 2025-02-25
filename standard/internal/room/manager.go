package room

import (
	"sync"
	"time"

	"github.com/gianglt2198/webrtc-tutorial/internal/peer"
	"github.com/google/uuid"
)

type Room struct {
	ID        string
	Peers     map[string]*peer.Peer
	Lock      sync.RWMutex
	CreatedAt time.Time
}

type Manager struct {
	rooms map[string]*Room
	sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

func (m *Manager) CreateRoom() string {
	id := uuid.New().String()
	m.rooms[id] = &Room{
		ID:        id,
		Peers:     make(map[string]*peer.Peer),
		CreatedAt: time.Now(),
	}
	return id
}
