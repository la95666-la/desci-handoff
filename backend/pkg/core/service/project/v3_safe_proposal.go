package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

const v3SafeService = "https://api.safe.global/tx-service/bnb/api/v1"
const v3SafeZero = "0x0000000000000000000000000000000000000000"

var v3SafeProposalMu sync.Mutex
var v3SafeProposalHTTP = &http.Client{
	Timeout: 8 * time.Second,
	// Never forward the bearer credential to a redirect target.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Only Safe owners sign, via their own wallet. No backend key can sign or execute.
type V3SafeProposal struct {
	Safe       string `json:"safe"`
	To         string `json:"to"`
	Data       string `json:"data"`
	Nonce      string `json:"nonce"`
	SafeTxHash string `json:"safe_tx_hash"`
	Sender     string `json:"sender,omitempty"`
	Signature  string `json:"signature,omitempty"`
}

func v3SafeCall(ctx context.Context, rpc *ethclient.Client, safe common.Address, signature string, args ...[]byte) ([]byte, error) {
	data := append([]byte{}, crypto.Keccak256([]byte(signature))[:4]...)
	for _, arg := range args {
		data = append(data, arg...)
	}
	return rpc.CallContract(ctx, ethereum.CallMsg{To: &safe, Data: data}, nil)
}

// Compute the hash on the actual Safe, not from a client-supplied nonce or digest.
func prepareV3SafeProposal(ctx context.Context, rpc *ethclient.Client, safe, target common.Address, data []byte) (V3SafeProposal, error) {
	invalid := errors.New("V3 Safe proposal unavailable")
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain.Cmp(big.NewInt(56)) != 0 || safe == (common.Address{}) || target != common.HexToAddress(v3TokenProxy) || len(data) < 4 {
		return V3SafeProposal{}, invalid
	}
	word, err := v3SafeCall(ctx, rpc, safe, "nonce()")
	if err != nil || len(word) != 32 {
		return V3SafeProposal{}, invalid
	}
	nonce := new(big.Int).SetBytes(word)
	// getTransactionHash(address,uint256,bytes,uint8,uint256,uint256,uint256,address,address,uint256)
	// All gas/refund fields and value are zero; operation is CALL.
	args := make([]byte, 320)
	copy(args[12:32], target.Bytes())
	copy(args[64:96], common.LeftPadBytes(big.NewInt(320).Bytes(), 32))
	copy(args[288:320], common.LeftPadBytes(nonce.Bytes(), 32))
	padded := (len(data) + 31) / 32 * 32
	args = append(args, common.LeftPadBytes(big.NewInt(int64(len(data))).Bytes(), 32)...)
	args = append(args, data...)
	args = append(args, make([]byte, padded-len(data))...)
	hash, err := v3SafeCall(ctx, rpc, safe, "getTransactionHash(address,uint256,bytes,uint8,uint256,uint256,uint256,address,address,uint256)", args)
	if err != nil || len(hash) != 32 || common.BytesToHash(hash) == (common.Hash{}) {
		return V3SafeProposal{}, invalid
	}
	return V3SafeProposal{Safe: safe.Hex(), To: target.Hex(), Data: hexutil.Encode(data), Nonce: nonce.String(), SafeTxHash: common.BytesToHash(hash).Hex()}, nil
}

func v3SafeOwnerSignature(ctx context.Context, rpc *ethclient.Client, proposal V3SafeProposal, sender, signature string) (string, error) {
	invalid := errors.New("Safe owner signature invalid")
	if !common.IsHexAddress(sender) || common.HexToAddress(sender) == (common.Address{}) || !v3CanonicalHash(proposal.SafeTxHash) {
		return "", invalid
	}
	sig, err := hexutil.Decode(signature)
	if err != nil || len(sig) != 65 {
		return "", invalid
	}
	v := sig[64]
	// Wallet personal_sign(hash) uses the Safe eth_sign mode (v=31/32), not a raw EIP-712 signature.
	if v == 27 || v == 28 {
		sig[64] = v - 27
	} else if v != 0 && v != 1 {
		return "", invalid
	}
	hash := crypto.Keccak256([]byte("\x19Ethereum Signed Message:\n32"), common.HexToHash(proposal.SafeTxHash).Bytes())
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil || crypto.PubkeyToAddress(*pub) != common.HexToAddress(sender) {
		return "", invalid
	}
	isOwner, err := v3SafeCall(ctx, rpc, common.HexToAddress(proposal.Safe), "isOwner(address)", common.LeftPadBytes(common.HexToAddress(sender).Bytes(), 32))
	if err != nil || len(isOwner) != 32 || new(big.Int).SetBytes(isOwner).Cmp(big.NewInt(1)) != 0 {
		return "", invalid
	}
	sig[64] += 31
	return hexutil.Encode(sig), nil
}

func v3SafeAPI(ctx context.Context, method, path string, body any) (int, []byte, error) {
	key := strings.TrimSpace(os.Getenv("SAFE_API_KEY"))
	if key == "" {
		return 0, nil, errors.New("Safe API disabled")
	}
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, v3SafeService+path, payload)
	if err != nil {
		return 0, nil, errors.New("Safe API unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := v3SafeProposalHTTP.Do(req)
	if err != nil {
		return 0, nil, errors.New("Safe API unavailable; check original hash before retry")
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, errors.New("Safe API response unavailable")
	}
	return resp.StatusCode, b, nil
}

func v3SafeQueueCheck(ctx context.Context, p V3SafeProposal) error {
	invalid := errors.New("Safe queue unavailable or nonce occupied; check Safe before retry")
	path := "/multisig-transactions/" + p.SafeTxHash + "/"
	code, _, err := v3SafeAPI(ctx, http.MethodGet, path, nil)
	if err != nil || code != http.StatusNotFound {
		return invalid
	}
	// The service defaults to trusted transactions only. Check both sides;
	// refuse any occupied nonce or a partial/undecodable response.
	for _, trusted := range []string{"true", "false"} {
		path := "/safes/" + p.Safe + "/multisig-transactions/?executed=false&nonce=" + p.Nonce + "&trusted=" + trusted + "&limit=100"
		code, body, err := v3SafeAPI(ctx, http.MethodGet, path, nil)
		if err != nil || code != http.StatusOK {
			return invalid
		}
		var list struct {
			Count   *int              `json:"count"`
			Results []json.RawMessage `json:"results"`
		}
		if json.Unmarshal(body, &list) != nil || list.Count == nil || *list.Count < 0 ||
			list.Results == nil || *list.Count != len(list.Results) {
			return invalid
		}
		// The request is filtered to this nonce. Even if the service ignores
		// that filter, refusing an unrelated row is safer than double booking.
		if *list.Count != 0 {
			return invalid
		}
	}
	return nil
}

// A 201 is not proof that the transaction was indexed. Read the exact hash
// back; never report proposal success on an unknown or mismatched response.
func v3SafeProposalReadback(ctx context.Context, p V3SafeProposal) error {
	code, body, err := v3SafeAPI(ctx, http.MethodGet, "/multisig-transactions/"+p.SafeTxHash+"/", nil)
	if err != nil || code != http.StatusOK {
		return errors.New("Safe proposal indexing unknown; check original hash")
	}
	var row struct {
		Safe       string      `json:"safe"`
		To         string      `json:"to"`
		Data       string      `json:"data"`
		Nonce      json.Number `json:"nonce"`
		SafeTxHash string      `json:"safeTxHash"`
		Value      json.Number `json:"value"`
		Operation  int         `json:"operation"`
	}
	if json.Unmarshal(body, &row) != nil || !strings.EqualFold(row.Safe, p.Safe) ||
		!strings.EqualFold(row.To, p.To) || !strings.EqualFold(row.Data, p.Data) ||
		!strings.EqualFold(row.SafeTxHash, p.SafeTxHash) || row.Nonce.String() != p.Nonce ||
		row.Value.String() != "0" || row.Operation != 0 {
		return errors.New("Safe proposal readback mismatch; check original hash")
	}
	return nil
}

func submitV3SafeProposal(ctx context.Context, rpc *ethclient.Client, safe, target common.Address, calldata []byte, expectedHash, sender, signature string) (V3SafeProposal, error) {
	return submitV3SafeProposalChecked(ctx, rpc, safe, target, calldata, expectedHash, sender, signature, nil)
}

func submitV3SafeProposalChecked(ctx context.Context, rpc *ethclient.Client, safe, target common.Address, calldata []byte, expectedHash, sender, signature string, beforePost func() error) (V3SafeProposal, error) {
	v3SafeProposalMu.Lock()
	defer v3SafeProposalMu.Unlock()
	p, err := prepareV3SafeProposal(ctx, rpc, safe, target, calldata)
	if err != nil || !strings.EqualFold(p.SafeTxHash, expectedHash) {
		return V3SafeProposal{}, errors.New("Safe nonce or payload changed; sign a fresh hash")
	}
	sig, err := v3SafeOwnerSignature(ctx, rpc, p, sender, signature)
	if err != nil {
		return V3SafeProposal{}, err
	}
	if err := v3SafeQueueCheck(ctx, p); err != nil {
		return V3SafeProposal{}, err
	}
	current, err := prepareV3SafeProposal(ctx, rpc, safe, target, calldata)
	if err != nil || current.SafeTxHash != p.SafeTxHash || current.Nonce != p.Nonce {
		return V3SafeProposal{}, errors.New("Safe nonce changed; check original hash")
	}
	if beforePost != nil {
		if err := beforePost(); err != nil {
			return V3SafeProposal{}, err
		}
	}
	body := map[string]any{"to": p.To, "value": "0", "data": p.Data, "operation": 0, "safeTxGas": "0", "baseGas": "0", "gasPrice": "0", "gasToken": v3SafeZero, "refundReceiver": v3SafeZero, "nonce": p.Nonce, "contractTransactionHash": p.SafeTxHash, "sender": common.HexToAddress(sender).Hex(), "signature": sig, "origin": "JoinCare V3"}
	code, _, err := v3SafeAPI(ctx, http.MethodPost, "/safes/"+p.Safe+"/multisig-transactions/", body)
	if err != nil || code != http.StatusCreated {
		return V3SafeProposal{}, fmt.Errorf("Safe proposal result unknown; query original Safe hash %s before retry", p.SafeTxHash)
	}
	if err := v3SafeProposalReadback(ctx, p); err != nil {
		return V3SafeProposal{}, fmt.Errorf("%w: %s", err, p.SafeTxHash)
	}
	p.Sender = common.HexToAddress(sender).Hex()
	return p, nil
}

func v3PolicyProposalRPC(ctx context.Context) (*ethclient.Client, error) {
	url := strings.TrimSpace(viper.GetString("chain.project.url"))
	if url == "" {
		return nil, errors.New("V3 policy RPC unavailable")
	}
	rpc, err := chainrpc.Dial(ctx, url)
	if err != nil {
		return nil, errors.New("V3 policy RPC unavailable")
	}
	return rpc, nil
}

func V3TokenSafeProposal(asset string, signed *V3SafeProposal) (V3SafeProposal, error) {
	invalid := errors.New("V3 token proposal unavailable")
	status, err := ReadV3TokenGateStatus()
	if err != nil || status == nil || status.ChainID != 56 || !strings.EqualFold(status.Proxy, v3TokenProxy) || !validNonZeroAddress(status.OwnerSafe) || status.SafeThreshold != 2 || status.Confirmations < 15 {
		return V3SafeProposal{}, invalid
	}
	var token V3TokenGateAsset
	switch asset {
	case "usdt":
		token = status.USDT
	case "testcat":
		if !status.USDT.Allowed {
			return V3SafeProposal{}, invalid
		}
		token = status.TESTCAT
	default:
		return V3SafeProposal{}, invalid
	}
	address, _, _, method, _, err := v3TokenAsset(asset)
	if err != nil || token.Allowed || token.Decimals != 18 || token.Simulation != "passed" || !strings.EqualFold(token.Token, address.Hex()) || token.Method != method {
		return V3SafeProposal{}, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return V3SafeProposal{}, invalid
	}
	defer rpc.Close()
	data := v3TokenCallData(method, &address, true)
	safe, err := v3CurrentOwnerAt(ctx, rpc, nil)
	if err != nil || safe != common.HexToAddress(status.OwnerSafe) {
		return V3SafeProposal{}, invalid
	}
	target := common.HexToAddress(v3TokenProxy)
	if signed == nil {
		proposal, err := prepareV3SafeProposal(ctx, rpc, safe, target, data)
		if err != nil {
			return V3SafeProposal{}, err
		}
		if err := v3SafeQueueCheck(ctx, proposal); err != nil {
			return V3SafeProposal{}, err
		}
		return proposal, nil
	}
	return submitV3SafeProposalChecked(ctx, rpc, safe, target, data, signed.SafeTxHash, signed.Sender, signed.Signature, func() error {
		return recheckV3OwnerProposal(ctx, rpc, safe)
	})
}

// A new project token is not promoted by an Admin request: the pinned Owner
// Safe must still sign and execute configureProjectToken for this address.
func V3ProjectTokenSafeProposal(address string, signed *V3SafeProposal) (V3SafeProposal, error) {
	invalid := errors.New("V3 project token proposal unavailable")
	status, err := ReadV3ProjectTokenGateStatus(address)
	if err != nil || status == nil || status.ChainID != 56 || status.Confirmations < 15 ||
		!strings.EqualFold(status.Proxy, v3TokenProxy) || !validNonZeroAddress(status.OwnerSafe) ||
		status.Asset.Allowed || status.Asset.Simulation != "passed" || !strings.EqualFold(status.Asset.Token, address) {
		return V3SafeProposal{}, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return V3SafeProposal{}, invalid
	}
	defer rpc.Close()
	token := common.HexToAddress(address)
	data := v3TokenCallData("configureProjectToken(address,bool)", &token, true)
	safe, err := v3CurrentOwnerAt(ctx, rpc, nil)
	if err != nil || safe != common.HexToAddress(status.OwnerSafe) {
		return V3SafeProposal{}, invalid
	}
	target := common.HexToAddress(v3TokenProxy)
	if signed == nil {
		proposal, err := prepareV3SafeProposal(ctx, rpc, safe, target, data)
		if err != nil {
			return V3SafeProposal{}, err
		}
		if err := v3SafeQueueCheck(ctx, proposal); err != nil {
			return V3SafeProposal{}, err
		}
		return proposal, nil
	}
	return submitV3SafeProposalChecked(ctx, rpc, safe, target, data, signed.SafeTxHash, signed.Sender, signed.Signature, func() error {
		return recheckV3OwnerProposal(ctx, rpc, safe)
	})
}

func recheckV3OwnerProposal(ctx context.Context, rpc *ethclient.Client, safe common.Address) error {
	owner, e := v3CurrentOwnerAt(ctx, rpc, nil)
	if e != nil || owner != safe {
		return errors.New("Owner Safe changed; refresh proposal")
	}
	_, _, e = v3TokenSafeAt(ctx, rpc, nil)
	return e
}

func V3PolicySafeProposal(projectID, reviewID, adminID uint64, signed *V3SafeProposal) (V3SafeProposal, error) {
	invalid := errors.New("V3 policy proposal unavailable")
	handoff, err := ReadV3PolicySafeHandoff(projectID, reviewID, adminID)
	if err != nil || handoff.ChainID != "56" || !strings.EqualFold(handoff.Proxy, v3TokenProxy) || !common.IsHexAddress(handoff.Governor) {
		return V3SafeProposal{}, invalid
	}
	data, err := hexutil.Decode(handoff.Calldata)
	if err != nil || len(data) != 132 {
		return V3SafeProposal{}, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	rpc, err := v3PolicyProposalRPC(ctx)
	if err != nil {
		return V3SafeProposal{}, invalid
	}
	defer rpc.Close()
	// Governor is the separately approved policy Safe; never borrow Owner Safe authority.
	safe := common.HexToAddress(handoff.Governor)
	target := common.HexToAddress(handoff.Proxy)
	if signed == nil {
		proposal, err := prepareV3SafeProposal(ctx, rpc, safe, target, data)
		if err != nil {
			return V3SafeProposal{}, err
		}
		if err := v3SafeQueueCheck(ctx, proposal); err != nil {
			return V3SafeProposal{}, err
		}
		return proposal, nil
	}
	// Approval, review expiry and the pinned on-chain version must still hold
	// at submission, not merely when the operator fetched the preview.
	fresh, err := ReadV3PolicySafeHandoff(projectID, reviewID, adminID)
	if err != nil || fresh != handoff {
		return V3SafeProposal{}, invalid
	}
	return submitV3SafeProposal(ctx, rpc, safe, target, data, signed.SafeTxHash, signed.Sender, signed.Signature)
}

// ApprovedV3PolicySafeProposal is an unrouted candidate. It only proposes an
// already four-eyes-approved policy call to the actual Governor Safe. No
// backend key signs or executes the Safe transaction, and an unknown result
// must be recovered by the original Safe hash rather than re-proposed.
func ApprovedV3PolicySafeProposal(projectID, reviewID, adminID uint64, signed *V3SafeProposal) (V3SafeProposal, error) {
	return ApprovedV3PolicySafeProposalContext(context.Background(), projectID, reviewID, adminID, signed)
}

func ApprovedV3PolicySafeProposalContext(parent context.Context, projectID, reviewID, adminID uint64, signed *V3SafeProposal) (result V3SafeProposal, resultErr error) {
	phase := "safe_handoff"
	var diagnosticCtx context.Context
	var cancel context.CancelFunc
	defer func() {
		if cancel != nil {
			defer cancel()
		}
		if resultErr != nil {
			resultErr = v3PolicyContextDiagnostic(diagnosticCtx, resultErr, phase)
		}
	}()
	invalid := errors.New("V3 approved policy proposal unavailable")
	if parent == nil || parent.Err() != nil {
		return V3SafeProposal{}, invalid
	}
	ctx, c := context.WithTimeout(parent, v3PolicyProposalBudget)
	cancel = c
	diagnosticCtx = ctx
	permission := middleware.ActRead
	if signed != nil {
		permission = middleware.ActExecute
	}
	handoff, err := readApprovedV3PolicyHandoffDB(ctx, db.Mysql(), projectID, reviewID, adminID, permission)
	if err != nil || handoff.ChainID != "56" || !strings.EqualFold(handoff.Proxy, v3TokenProxy) ||
		!strings.EqualFold(handoff.Governor, v3DiagnosticGovernor) {
		if err != nil {
			return V3SafeProposal{}, err
		}
		return V3SafeProposal{}, invalid
	}
	data, err := hexutil.Decode(handoff.Calldata)
	if err != nil || len(data) != 132 {
		return V3SafeProposal{}, invalid
	}
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return V3SafeProposal{}, invalid
	}
	defer rpc.Close()
	safe, target := common.HexToAddress(v3DiagnosticGovernor), common.HexToAddress(v3TokenProxy)
	_, governorCode, err := approvedV3SafeHashes()
	if err != nil {
		return V3SafeProposal{}, err
	}
	if signed == nil {
		phase = "policy_simulation"
		if err := simulateV3PolicyHandoff(ctx, rpc, handoff, governorCode); err != nil {
			return V3SafeProposal{}, err
		}
		phase = "safe_proposal"
		proposal, err := prepareV3SafeProposal(ctx, rpc, safe, target, data)
		if err != nil {
			return V3SafeProposal{}, invalid
		}
		if v3SafeQueueCheck(ctx, proposal) != nil {
			return V3SafeProposal{}, invalid
		}
		return proposal, nil
	}
	// One final DB-only recheck and simulation immediately before Safe POST.
	if !v3CanonicalHash(signed.SafeTxHash) {
		return V3SafeProposal{}, invalid
	}
	phase = "safe_service"
	return submitV3SafeProposalChecked(ctx, rpc, safe, target, data, signed.SafeTxHash, signed.Sender, signed.Signature, func() error {
		latest, e := readApprovedV3PolicyHandoffDB(ctx, db.Mysql(), projectID, reviewID, adminID, middleware.ActExecute)
		if e != nil {
			return e
		}
		if latest != handoff {
			return errors.New("V3 approved proposal content changed")
		}
		return simulateV3PolicyHandoff(ctx, rpc, latest, governorCode)
	})
}
