package action

import (
	"encoding/json/v2"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/auth/sign"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/notice/model"
	"joincare_server/pkg/notice/repository/connection"
)

func OnInit(stopCh chan struct{}) {
	connection.InitCache(stopCh)
	go OnExit(stopCh)
}

func OnExit(stopCh chan struct{}) {
	for {
		select {
		case _, ok := <-stopCh:
			if !ok {
				connection.Clear()
				return
			}
		}
	}
}

func OnOpen(conn *websocket.Conn, addr string) {
	conn.SetCloseHandler(func(code int, text string) error {
		OnCloseConnection(addr, conn)
		return nil
	})
	connection.AddConnection(addr, conn)
}

func OnClose(code int, text string, addr string) {
	connection.RemoveConnection(addr)
	connection.RemoveUser(addr)
}

func OnCloseConnection(addr string, conn *websocket.Conn) {
	if connection.RemoveConnectionIfMatch(addr, conn) {
		connection.RemoveUser(addr)
	}
}

func ReceiveMessage(addr string, body []byte) {
	msg := model.Message{}
	err := json.Unmarshal(body, &msg)
	if err != nil {
		log.GetLogger().Errorf("Receive Message Unmarshal err: %v", err)
		OnClose(1, "unmarshal err", addr)
		return
	}
	info, err := sign.ParseToken(msg.Token, viper.GetString("jwt.secret"))
	if err != nil {
		OnClose(1, "token check err", addr)
		return
	}
	switch msg.Event {
	case model.EventHeartbeat:
		heartbeat(info.UID)
	case model.EventLogin:
		login(addr, info.UID)
	}
}
