package model

import "encoding/json"

type Message struct {
	Time  int64           `json:"time"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
	UID   int             `json:"uid"`
	Token string          `json:"token,omitempty"`
}

const (
	EventHeartbeat = "heartbeat"
	EventLogin     = "login"
)
