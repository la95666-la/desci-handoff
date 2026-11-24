package idov3whitelist

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateApprovedV3RPCEndpoint is a shared Core/Job allowlist for the V3
// commercial read endpoint. Never return the URL or a provider error: the
// path contains a credential. This is not a relayer transport configuration.
func ValidateApprovedV3RPCEndpoint(raw string) error {
	invalid := errors.New("approved V3 read RPC unavailable")
	if raw == "" || raw != strings.TrimSpace(raw) || len(raw) > 2048 {
		return invalid
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" ||
		(u.Hostname() != "silent-bold-vineyard.bsc.quiknode.pro" &&
			u.Hostname() != "powerful-multi-telescope.bsc.quiknode.pro" &&
			u.Hostname() != "light-wild-meme.bsc.quiknode.pro") || u.Port() != "" ||
		u.Path == "" || u.Path == "/" || u.RawQuery != "" || u.Fragment != "" ||
		u.User != nil || u.Opaque != "" || u.RawPath != "" {
		return invalid
	}
	return nil
}
