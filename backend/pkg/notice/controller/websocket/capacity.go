package websocket

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

const (
	defaultMaxConnections      = 5000
	defaultMaxConnectionsPerIP = 20
)

type capacityLimiter struct {
	mu       sync.Mutex
	total    int
	byIP     map[string]int
	maxTotal int
	maxPerIP int
}

func newCapacityLimiter(maxTotal, maxPerIP int) *capacityLimiter {
	if maxTotal <= 0 {
		maxTotal = defaultMaxConnections
	}
	if maxPerIP <= 0 {
		maxPerIP = defaultMaxConnectionsPerIP
	}
	return &capacityLimiter{byIP: make(map[string]int), maxTotal: maxTotal, maxPerIP: maxPerIP}
}

func (l *capacityLimiter) reserve(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.maxTotal || l.byIP[ip] >= l.maxPerIP {
		return false
	}
	l.total++
	l.byIP[ip]++
	return true
}

func (l *capacityLimiter) release(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byIP[ip] <= 0 {
		return
	}
	l.total--
	l.byIP[ip]--
	if l.byIP[ip] == 0 {
		delete(l.byIP, ip)
	}
}

func parseTrustedProxyCIDRs(values []string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(strings.TrimSpace(value))
		if err == nil {
			result = append(result, network)
		}
	}
	return result
}

func clientIP(r *http.Request, trusted []*net.IPNet) string {
	peer := hostOnly(r.RemoteAddr)
	current := net.ParseIP(peer)
	if !ipInNetworks(current, trusted) {
		return peer
	}

	forwardedHeader := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	forwarded := strings.Split(forwardedHeader, ",")
	for i := len(forwarded) - 1; forwardedHeader != "" && i >= 0; i-- {
		candidateText := strings.TrimSpace(forwarded[i])
		candidate := net.ParseIP(candidateText)
		if candidate == nil {
			return peer
		}
		current = candidate
		if !ipInNetworks(current, trusted) {
			return candidateText
		}
	}
	if current != nil && !current.Equal(net.ParseIP(peer)) {
		return current.String()
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(real) != nil {
		return real
	}
	return peer
}

func hostOnly(address string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err == nil {
		return host
	}
	return strings.Trim(strings.TrimSpace(address), "[]")
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
