package controller

import (
	"encoding/json"
	"testing"
)

func TestValidateProtectedOnchainNamespaces(t *testing.T) {
	current := `{"marketing":{"website":"https://old.example"},"fairLaunch":{"saleTokenAmountRaw":"9007199254740993","softCapRaw":"1000000"}}`
	tests := []struct {
		name      string
		requested string
		wantError bool
	}{
		{
			name:      "preserves namespace while editing marketing",
			requested: `{"fairLaunch":{"softCapRaw":"1000000","saleTokenAmountRaw":"9007199254740993"},"marketing":{"website":"https://new.example"}}`,
		},
		{
			name:      "rejects changing protected value",
			requested: `{"fairLaunch":{"saleTokenAmountRaw":"9007199254740994","softCapRaw":"1000000"}}`,
			wantError: true,
		},
		{
			name:      "rejects removing protected namespace",
			requested: `{"marketing":{}}`,
			wantError: true,
		},
		{
			name:      "rejects adding protected namespace",
			requested: `{"fairLaunch":{"saleTokenAmountRaw":"1"}}`,
			wantError: true,
		},
		{
			name:      "rejects numeric precision collision",
			requested: `{"fairLaunch":{"saleTokenAmountRaw":9007199254740993}}`,
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := current
			if test.name == "rejects adding protected namespace" {
				base = `{"marketing":{}}`
			} else if test.name == "rejects numeric precision collision" {
				base = `{"fairLaunch":{"saleTokenAmountRaw":9007199254740992}}`
			}
			err := validateProtectedOnchainNamespaces(base, json.RawMessage(test.requested))
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
		})
	}
}

func TestV3OnchainNamespacesCannotBeForgedByMarketingEditor(t *testing.T) {
	for _, namespace := range []string{"idoV3", "ido_v3"} {
		t.Run(namespace, func(t *testing.T) {
			current := `{"marketing":{"title":"before"},"` + namespace + `":{"version":"9007199254740993"}}`
			for name, pair := range map[string][2]string{
				"preserve":  {current, `{"` + namespace + `":{"version":"9007199254740993"},"marketing":{"title":"after"}}`},
				"add":       {`{"marketing":{}}`, `{"marketing":{},"` + namespace + `":{"version":1}}`},
				"remove":    {current, `{"marketing":{}}`},
				"change":    {current, `{"` + namespace + `":{"version":"9007199254740994"}}`},
				"precision": {`{"` + namespace + `":{"version":9007199254740992}}`, `{"` + namespace + `":{"version":9007199254740993}}`},
			} {
				t.Run(name, func(t *testing.T) {
					err := validateProtectedOnchainNamespaces(pair[0], json.RawMessage(pair[1]))
					if (err == nil) != (name == "preserve") {
						t.Fatalf("unexpected protected namespace validation: %v", err)
					}
				})
			}
		})
	}
}
