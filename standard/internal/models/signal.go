package models

import "encoding/json"

type SignalMessage struct {
	Type     string          `json:"type"`
	RoomID   string          `json:"room_id,omitempty"`
	SenderID string          `json:"sender_id,omitempty"`
	TargetID string          `json:"target_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type DirectSignalMessage struct {
	Type     string          `json:"type"`
	RoomID   string          `json:"room_id"`
	SenderID string          `json:"sender_id"`
	TargetID string          `json:"target_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Peers    []string        `json:"peers,omitempty"`
}
