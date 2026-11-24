package connection

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type socket interface {
	Close() error
	SetWriteDeadline(time.Time) error
	WriteJSON(any) error
}

type connectionEntry struct {
	conn    socket
	writeMu sync.Mutex
}

type Engine struct {
	StopCh      chan struct{}
	Connections Connections
	UserInfos   UserInfos
}

type Connections struct {
	Set map[string]*connectionEntry
	sync.RWMutex
}

type UserInfos struct {
	Set map[int]*UserInfo
	sync.RWMutex
}

type UserInfo struct {
	Uid           int
	Address       string
	LastHeartBeat int64
}

var _ socket = (*websocket.Conn)(nil)
