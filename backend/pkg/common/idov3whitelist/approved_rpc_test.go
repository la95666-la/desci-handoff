package idov3whitelist

import "testing"

func TestApprovedV3RPCIsRestrictedToCommercialReadProvider(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		valid    bool
	}{
		{"https://silent-bold-vineyard.bsc.quiknode.pro/placeholder-token/", true},
		{"https://powerful-multi-telescope.bsc.quiknode.pro/placeholder-token/", true},
		{"https://light-wild-meme.bsc.quiknode.pro/placeholder-token/", true},
		{"http://light-wild-meme.bsc.quiknode.pro/placeholder-token/", false},
		{"https://light-wild-meme.bsc.quiknode.pro/", false},
		{"https://light-wild-meme.bsc.quiknode.pro:443/placeholder-token/", false},
		{"https://light-wild-meme.bsc.quiknode.pro/placeholder-token/?secret=1", false},
		{"https://light-wild-meme.bsc.quiknode.pro/placeholder-token/#fragment", false},
		{"https://user:password@light-wild-meme.bsc.quiknode.pro/placeholder-token/", false},
		{"https://light-wild-meme.bsc.quiknode.pro.evil.example/placeholder-token/", false},
		{"http://powerful-multi-telescope.bsc.quiknode.pro/placeholder-token/", false},
		{"https://powerful-multi-telescope.bsc.quiknode.pro/", false},
		{"https://powerful-multi-telescope.bsc.quiknode.pro/placeholder-token/?x=1", false},
		{"https://powerful-multi-telescope.bsc.quiknode.pro:443/placeholder-token/", false},
		{"http://silent-bold-vineyard.bsc.quiknode.pro/placeholder-token/", false},
		{"https://silent-bold-vineyard.bsc.quiknode.pro/", false},
		{"https://silent-bold-vineyard.bsc.quiknode.pro/placeholder-token/?x=1", false},
		{"https://silent-bold-vineyard.bsc.quiknode.pro:443/placeholder-token/", false},
		{"https://other.example/placeholder-token/", false},
		{"https://user:password@silent-bold-vineyard.bsc.quiknode.pro/placeholder-token/", false},
		{" https://silent-bold-vineyard.bsc.quiknode.pro/placeholder-token/", false},
	} {
		if err := ValidateApprovedV3RPCEndpoint(tc.endpoint); (err == nil) != tc.valid {
			t.Fatalf("provider acceptance mismatch: valid=%v err=%v", tc.valid, err)
		}
	}
}
