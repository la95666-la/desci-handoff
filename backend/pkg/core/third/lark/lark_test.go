package lark

import (
	"net"
	"testing"
)

func TestForbiddenEgressIPRejectsSpecialUseNetworks(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "198.18.0.1", "192.0.2.1", "203.0.113.1", "2001:db8::1", "::1"} {
		if !forbiddenEgressIP(net.ParseIP(raw)) {
			t.Fatalf("special-use egress address accepted: %s", raw)
		}
	}
	if forbiddenEgressIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("ordinary public address was rejected")
	}
}
