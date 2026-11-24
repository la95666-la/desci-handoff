package websocket

import (
	"net/http"
	"time"
)

func Start(host string) *http.Server {
	httpServer := &http.Server{
		Addr:              host,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	httpServer.SetKeepAlivesEnabled(true) // true recommended
	go httpServer.ListenAndServe()
	return httpServer
}
