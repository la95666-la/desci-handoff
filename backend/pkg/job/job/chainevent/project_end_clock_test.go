package chainevent

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	baseChain "joincare_server/pkg/common/chain"
)

func TestConfirmedV2EndTimeUsesSafeBlockNotHostClock(t *testing.T) {
	requests := make([]string, 0, 2)
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
			return
		}
		requests = append(requests, in.Method)
		var payload string
		switch in.Method {
		case "eth_blockNumber":
			payload = `"0xa"`
		case "eth_getBlockByNumber":
			if len(in.Params) < 1 || string(in.Params[0]) != `"0x8"` {
				t.Errorf("used unsafe head: %s", in.Params)
				return
			}
			header, marshalErr := json.Marshal(&types.Header{Number: big.NewInt(8), Time: uint64(time.Now().Add(2 * time.Hour).Unix()), Difficulty: big.NewInt(0)})
			if marshalErr != nil {
				t.Error(marshalErr)
				return
			}
			payload = string(header)
		default:
			t.Errorf("unexpected RPC %s", in.Method)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, in.ID, payload)
	}))
	defer rpc.Close()
	key, err := crypto.HexToECDSA("1111111111111111111111111111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	client, err := baseChain.NewIDOV2Client(rpc.URL, key, common.HexToAddress("0x2222222222222222222222222222222222222222"), big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	client.Confirmations = 2
	got, err := confirmedV2EndTime(context.Background(), client)
	if err != nil || !got.After(time.Now().Add(time.Hour)) || len(requests) != 2 {
		t.Fatalf("confirmed clock=%v err=%v requests=%v", got, err, requests)
	}
	client.Confirmations = 10
	if _, err := confirmedV2EndTime(context.Background(), client); err == nil {
		t.Fatal("admitted V2 end without a confirmed block")
	}
}
