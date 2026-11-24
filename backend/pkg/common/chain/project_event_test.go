package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
)

func TestProcessLogWithoutTopicsReturnsNil(t *testing.T) {
	client := &Client{}
	if data := client.ProcessLog(types.Log{}); data != nil {
		t.Fatalf("log without topics must be ignored, got %+v", data)
	}
}
