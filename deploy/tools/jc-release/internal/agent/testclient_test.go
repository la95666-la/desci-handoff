package agent

import "net/http"

// NewTestClientAllowLoopbackHTTP is compiled only with tests because this file
// is a _test.go source. Production packages expose no plain-HTTP constructor.
func NewTestClientAllowLoopbackHTTP(baseURL, token string, client *http.Client) (*Client, error) {
	return newClient(baseURL, token, client)
}
