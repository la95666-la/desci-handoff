package chaintx

import (
	"context"
	"net/http"
	"testing"
)

type rejectTransport struct{ called bool }

func (r *rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.called = true
	return nil, context.Canceled
}
func TestHTTPGatewayRequiresHTTPS(t *testing.T) {
	transport := &rejectTransport{}
	g := HTTPGateway{BaseURL: "http://gateway.local", Client: &http.Client{Transport: transport}}
	if _, err := g.Journal(context.Background(), GatewayJournalKey{}); err == nil {
		t.Fatal("http accepted")
	}
	if transport.called {
		t.Fatal("network attempted before URL validation")
	}
}
