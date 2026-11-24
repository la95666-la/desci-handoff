package project

import (
	"encoding/json/v2"
	"testing"
)

func TestSubmitProjectReviewAmountsAcceptNumbersAndStringsWithoutFloatConversion(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "JSON number", body: `{"projectID":"1","tokenPrice":1.000001,"hardCap":1000.5,"softCap":500,"maxPerUser":100}`, want: "1.000001"},
		{name: "JSON string", body: `{"projectID":"1","tokenPrice":"1.000001","hardCap":"1000.5","softCap":"500","maxPerUser":"100"}`, want: "1.000001"},
		{name: "long exact string", body: `{"projectID":"1","tokenPrice":"123456789012345678.123456789012345678","hardCap":"1000","softCap":"500","maxPerUser":"100"}`, want: "123456789012345678.123456789012345678"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var req SubmitProjectReviewRequest
			if err := json.Unmarshal([]byte(test.body), &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if req.TokenPrice.String() != test.want {
				t.Fatalf("token price=%s want=%s", req.TokenPrice.String(), test.want)
			}
		})
	}
}
