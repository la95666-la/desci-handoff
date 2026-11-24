package websocket

import (
	"fmt"
	"net/http"

	"github.com/spf13/viper"

	"joincare_server/pkg/common/websocket"
	wsHandle "joincare_server/pkg/notice/controller/websocket"
	wsService "joincare_server/pkg/notice/service/action"
)

func Start(stopCh chan struct{}) *http.Server {
	wsHandle.InitUpGrader()
	http.HandleFunc("/", wsHandle.WsHandler)
	wsService.OnInit(stopCh)
	host := fmt.Sprintf("%s:%d", viper.GetString("protocol.websocket.host"), viper.GetInt("protocol.websocket.port"))
	ser := websocket.Start(host)
	return ser
}
