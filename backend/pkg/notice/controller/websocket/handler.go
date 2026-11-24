package websocket

import (
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	commonserver "joincare_server/pkg/common/http/server"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/notice/service/action"
)

const (
	maxMessageBytes = 64 << 10
	pongWait        = 90 * time.Second
)

var (
	upGrader       websocket.Upgrader
	capacity       *capacityLimiter
	trustedProxies []*net.IPNet
)

func InitUpGrader() {
	upGrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || commonserver.AllowedOrigin(origin)
		},
		HandshakeTimeout:  10 * time.Second,
		EnableCompression: true,
	}
	capacity = newCapacityLimiter(
		viper.GetInt("protocol.websocket.maxConnections"),
		viper.GetInt("protocol.websocket.maxConnectionsPerIP"),
	)
	trustedProxies = parseTrustedProxyCIDRs(viper.GetStringSlice("protocol.websocket.trustedProxyCIDRs"))
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, trustedProxies)
	if !capacity.reserve(ip) {
		w.Header().Set("Retry-After", "5")
		http.Error(w, "websocket capacity exceeded", http.StatusServiceUnavailable)
		return
	}
	defer capacity.release(ip)

	conn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	addr := conn.RemoteAddr().String()
	action.OnOpen(conn, addr)
	defer action.OnCloseConnection(addr, conn)
	conn.SetReadLimit(maxMessageBytes)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, bs, readErr := conn.ReadMessage()
		if readErr != nil {
			log.GetLogger().Errorf("read data err: %s", readErr)
			return
		}
		action.ReceiveMessage(addr, bs)
	}
}
