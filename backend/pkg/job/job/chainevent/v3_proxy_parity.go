package chainevent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

type v3ParityProjection struct {
	State                                                           string
	RaisedRaw, SoldRaw, DepositedRaw, RefundedRaw, ClaimedRaw       string
	ProceedsPaidRaw, FeePaidRaw, ProjectTokensWithdrawnRaw, Wallets string
}

func auditV3ProjectParityInput(ctx context.Context, tx *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, id, tip uint64, hash common.Hash) (commonchain.V3ReplayTotals, error) {
	b, creation, e := v3ParityInput(ctx, tx, s, id)
	if e != nil {
		return commonchain.V3ReplayTotals{}, e
	}
	return auditV3CoveredProject(ctx, tx, rpc, s, b, creation, tip, hash, false)
}

// Parity inputs are not approval authority. Consumers still require original
// signed creation/approval admission before any publication.
func v3ParityInput(ctx context.Context, tx *gorm.DB, s v3StreamScope, id uint64) (idov3whitelist.ExpectedProjectBinding, uint64, error) {
	var row struct {
		ProjectToken, PaymentToken string
		CreationBlock              uint64
	}
	r := tx.WithContext(ctx).Raw(`SELECT p.token_address AS project_token,p.payment_token_address AS payment_token,j.block_number AS creation_block FROM projects p JOIN ido_v3_project_binding b ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND b.proxy_address=p.contract_address AND b.contract_version=p.contract_version JOIN ido_v3_event_journal j ON j.chain_id=b.chain_id AND j.proxy_address=b.proxy_address AND j.project_id=b.project_id AND j.contract_version=b.contract_version AND j.event_kind='created' AND j.status='canonical' WHERE b.chain_id=? AND b.proxy_address=? AND b.contract_version=? AND b.project_id=? AND p.onchain_locked=1`, append(s.args(), id)...).Scan(&row)
	b := idov3whitelist.ExpectedProjectBinding{ChainID: s.Chain, ProxyAddress: s.Proxy, ProjectID: id, ProjectToken: common.HexToAddress(row.ProjectToken), PaymentToken: common.HexToAddress(row.PaymentToken)}
	if r.Error != nil || r.RowsAffected != 1 || row.CreationBlock == 0 || b.ProjectToken == (common.Address{}) || b.PaymentToken == (common.Address{}) {
		return b, 0, errV3Stream
	}
	return b, row.CreationBlock, nil
}

func v3ExpectedProjection(t commonchain.V3ReplayTotals) (v3ParityProjection, error) {
	states := []string{"pending", "active", "successful", "failed", "cancelled"}
	if int(t.State) >= len(states) || t.Raised == nil || t.Sold == nil || t.Deposited == nil || t.Refunded == nil || t.Claimed == nil || t.ProceedsPaid == nil || t.FeePaid == nil || t.ProjectTokensWithdrawn == nil {
		return v3ParityProjection{}, errV3Stream
	}
	wallets := map[string]map[string]string{}
	for address, w := range t.Wallets {
		wallets[v3Lower(address)] = map[string]string{"payment": w.Payment.String(), "tokens": w.Tokens.String(), "refunded": w.Refunded.String(), "claimed": w.Claimed.String()}
	}
	raw, e := json.Marshal(wallets)
	if e != nil {
		return v3ParityProjection{}, e
	}
	return v3ParityProjection{State: states[t.State], RaisedRaw: t.Raised.String(), SoldRaw: t.Sold.String(), DepositedRaw: t.Deposited.String(), RefundedRaw: t.Refunded.String(), ClaimedRaw: t.Claimed.String(), ProceedsPaidRaw: t.ProceedsPaid.String(), FeePaidRaw: t.FeePaid.String(), ProjectTokensWithdrawnRaw: t.ProjectTokensWithdrawn.String(), Wallets: string(raw)}, nil
}

func v3ProjectionEqual(a, b v3ParityProjection) bool {
	x, xOK := v3ProjectionWallets(a.Wallets)
	y, yOK := v3ProjectionWallets(b.Wallets)
	if !xOK || !yOK || !reflect.DeepEqual(x, y) {
		return false
	}
	a.Wallets = ""
	b.Wallets = ""
	return a == b
}

// MySQL JSON rewrites whitespace/key order. Compare the exact wallet schema,
// not serialized bytes, and never decode monetary values through float64.
// Reject duplicate keys, unknown/missing fields, non-string and noncanonical
// uint256 amounts rather than normalizing potentially corrupt projections.
func v3ProjectionWallets(raw string) (map[string]map[string]string, bool) {
	d := json.NewDecoder(strings.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	wallets := map[string]map[string]string{}
	for d.More() {
		token, err = d.Token()
		address, ok := token.(string)
		if err != nil || !ok || !common.IsHexAddress(address) || address != v3Lower(common.HexToAddress(address)) || common.HexToAddress(address) == (common.Address{}) {
			return nil, false
		}
		if _, exists := wallets[address]; exists {
			return nil, false
		}
		token, err = d.Token()
		if err != nil || token != json.Delim('{') {
			return nil, false
		}
		wallet := map[string]string{}
		for d.More() {
			token, err = d.Token()
			key, ok := token.(string)
			if err != nil || !ok {
				return nil, false
			}
			switch key {
			case "payment", "tokens", "refunded", "claimed":
			default:
				return nil, false
			}
			if _, exists := wallet[key]; exists {
				return nil, false
			}
			token, err = d.Token()
			value, ok := token.(string)
			if err != nil || !ok {
				return nil, false
			}
			n, valid := new(big.Int).SetString(value, 10)
			if !valid || n.Sign() < 0 || n.BitLen() > 256 || n.String() != value {
				return nil, false
			}
			wallet[key] = value
		}
		token, err = d.Token()
		if err != nil || token != json.Delim('}') || len(wallet) != 4 {
			return nil, false
		}
		wallets[address] = wallet
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') {
		return nil, false
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, false
	}
	return wallets, true
}

type v3ParityEffect struct {
	BlockNumber                                                  uint64
	BlockHash, TxHash                                            string
	LogIndex                                                     uint
	EntryKind, AssetRole, WalletAddress, TokenAddress, AmountRaw string
}

func expectedV3Effects(b idov3whitelist.ExpectedProjectBinding, creation *commonchain.V3CreatedEvent, l types.Log, e commonchain.V3ReplayEvent) ([]v3ParityEffect, error) {
	base := v3ParityEffect{BlockNumber: l.BlockNumber, BlockHash: strings.ToLower(l.BlockHash.Hex()), TxHash: strings.ToLower(l.TxHash.Hex()), LogIndex: l.Index}
	makeEffect := func(kind, role string, wallet, token common.Address, raw string) v3ParityEffect {
		v := base
		v.EntryKind = kind
		v.AssetRole = role
		v.WalletAddress = v3Lower(wallet)
		v.TokenAddress = v3Lower(token)
		v.AmountRaw = raw
		return v
	}
	if e.Created != nil {
		return nil, nil
	}
	if e.Purchase != nil {
		return []v3ParityEffect{makeEffect("purchase", "payment", e.Purchase.Buyer, b.PaymentToken, e.Purchase.PaymentRaw), makeEffect("purchase", "project", e.Purchase.Buyer, b.ProjectToken, e.Purchase.ProjectTokenRaw)}, nil
	}
	if e.Lifecycle == nil {
		return nil, errV3Stream
	}
	v := e.Lifecycle
	switch v.Kind {
	case "ProjectActivated", "ProjectEnded", "ProjectCancelled":
		return nil, nil
	case "ProjectTokensDeposited":
		if creation == nil {
			return nil, errV3Stream
		}
		return []v3ParityEffect{makeEffect(v.Kind, "project", creation.ProjectOwner, b.ProjectToken, v.AmountRaw)}, nil
	case "Claimed", "ProjectTokensWithdrawn":
		return []v3ParityEffect{makeEffect(v.Kind, "project", v.Recipient, b.ProjectToken, v.AmountRaw)}, nil
	case "Refunded", "ProjectProceedsWithdrawn", "PlatformFeeWithdrawn":
		return []v3ParityEffect{makeEffect(v.Kind, "payment", v.Recipient, b.PaymentToken, v.AmountRaw)}, nil
	default:
		return nil, errV3Stream
	}
}

// Full-history parity, not endpoint-only parity. Coverage MUST include creation
// itself, not an empty bootstrap marker at that height. Every covered block,
// complete event set, journal payload, financial effect and policy fact is
// compared. This is read-only; no projection is rebuilt into live tables.
func auditV3CoveredProject(ctx context.Context, tx *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, creationBlock, tip uint64, tipHash common.Hash, halted bool) (commonchain.V3ReplayTotals, error) {
	var zero commonchain.V3ReplayTotals
	stream, e := v3StreamCursorRead(tx, s, true)
	if e != nil || stream.State != "running" || creationBlock <= stream.AnchorBlock || tip > stream.CursorBlock || tip < creationBlock {
		return zero, fmt.Errorf("%w: full creation coverage not ready", errV3Stream)
	}
	var rows []v3JournalRow
	if tx.WithContext(ctx).Raw(`SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND status='canonical' ORDER BY block_number,log_index`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion).Scan(&rows).Error != nil {
		return zero, errV3Stream
	}
	var replay []commonchain.V3ReplayEvent
	var effects []v3ParityEffect
	var creation *commonchain.V3CreatedEvent
	position := 0
	var lastHash common.Hash
	policyLogs := []types.Log{}
	for start := creationBlock; start <= tip; {
		end := start + 127
		if end < start || end > tip {
			end = tip
		}
		coverage, e := readV3Coverage(tx.WithContext(ctx), s, start-1, end)
		if e != nil {
			return zero, e
		}
		var checkpoints []struct {
			Number uint64 `gorm:"column:block_number"`
			Hash   string `gorm:"column:block_hash"`
		}
		read := tx.WithContext(ctx).Raw(`SELECT block_number,block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND status='canonical' AND block_number BETWEEN ? AND ? ORDER BY block_number`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion, start, end).Scan(&checkpoints)
		if read.Error != nil || len(checkpoints) != int(end-start+1) {
			return zero, errV3Stream
		}
		for n := start; n <= end; n++ {
			block := coverage[n]
			if lastHash != (common.Hash{}) && block.header().ParentHash != lastHash {
				return zero, errV3Stream
			}
			lastHash = block.header().Hash()
			checkpoint := checkpoints[n-start]
			if checkpoint.Number != n || checkpoint.Hash != strings.ToLower(lastHash.Hex()) {
				return zero, errV3Stream
			}
			for _, l := range block.Logs {
				if len(l.Topics) < 2 {
					continue
				}
				if new(big.Int).SetBytes(l.Topics[1][:]).Cmp(new(big.Int).SetUint64(b.ProjectID)) != 0 {
					continue
				}
				if l.Topics[0] == v3PolicyUpdateTopic {
					policyLogs = append(policyLogs, l)
					continue
				}
				if _, ok := v3ProjectEventTopics[l.Topics[0]]; !ok {
					continue
				}
				actual, e := commonchain.DecodeV3ReplayLog(l, b)
				if e != nil || position >= len(rows) {
					return zero, errV3Stream
				}
				row := rows[position]
				var stored commonchain.V3ReplayEvent
				kind := "created"
				if actual.Purchase != nil {
					kind = "purchase"
				}
				if actual.Lifecycle != nil {
					kind = actual.Lifecycle.Kind
				}
				if row.BlockNumber != n || row.BlockHash != strings.ToLower(l.BlockHash.Hex()) || row.TxHash != strings.ToLower(l.TxHash.Hex()) || row.LogIndex != l.Index || row.EventKind != kind || json.Unmarshal([]byte(row.Payload), &stored) != nil || !reflect.DeepEqual(actual, stored) {
					return zero, errV3Stream
				}
				if actual.Created != nil {
					if creation != nil || n != creationBlock || position != 0 {
						return zero, errV3Stream
					}
					creation = actual.Created
					if commonchain.MatchV3CreatedBinding(creation, b) != nil {
						return zero, errV3Stream
					}
				}
				if creation == nil {
					return zero, errV3Stream
				}
				receipt := (*types.Receipt)(block.Receipts[l.TxHash.Hex()])
				var match error
				switch {
				case actual.Created != nil:
					match = commonchain.MatchV3CreatedReceipt(actual.Created, b, receipt)
				case actual.Purchase != nil:
					match = commonchain.MatchV3PurchaseReceipt(actual.Purchase, commonchain.V3PurchaseBinding{ChainID: b.ChainID, ProxyAddress: b.ProxyAddress, ProjectID: b.ProjectID, PaymentToken: b.PaymentToken, ProjectToken: b.ProjectToken}, receipt)
				case actual.Lifecycle != nil:
					match = commonchain.MatchV3LifecycleReceipt(actual.Lifecycle, b, receipt)
				}
				if match != nil {
					return zero, errV3Stream
				}
				expected, e := expectedV3Effects(b, creation, l, actual)
				if e != nil {
					return zero, e
				}
				effects = append(effects, expected...)
				replay = append(replay, actual)
				position++
			}
		}
		if end == tip {
			break
		}
		start = end + 1
	}
	if position != len(rows) || creation == nil || lastHash != tipHash {
		return zero, errV3Stream
	}
	totals, e := commonchain.RebuildV3Replay(b, replay)
	if e != nil {
		return zero, e
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(s.Chain), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: s.Proxy}
	observed, e := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, s.Runtime, tip, tipHash)
	if e != nil || commonchain.MatchV3CreationProjectImmutable(creation, b, observed) != nil || commonchain.MatchV3ReplayProjectEndOfBlock(b, totals, observed, tip, tipHash) != nil {
		return zero, errV3Stream
	}
	var storedEffects []v3ParityEffect
	state := "canonical"
	if halted {
		state = "suspended"
	}
	if tx.Raw(`SELECT block_number,block_hash,tx_hash,log_index,entry_kind,asset_role,wallet_address,token_address,amount_raw FROM ido_v3_financial_effect WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND status=? ORDER BY block_number,log_index,asset_role`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion, state).Scan(&storedEffects).Error != nil {
		return zero, errV3Stream
	}
	if !v3EffectsEqual(effects, storedEffects) {
		return zero, fmt.Errorf("%w: financial parity mismatch", errV3Stream)
	}
	if !halted {
		var projection v3ParityProjection
		r := tx.Raw(`SELECT state,raised_raw,sold_raw,deposited_raw,refunded_raw,claimed_raw,proceeds_paid_raw,fee_paid_raw,project_tokens_withdrawn_raw,wallets FROM ido_v3_project_projection WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version=? AND cursor_block=? AND cursor_hash=?`, s.Chain, v3Lower(s.Proxy), b.ProjectID, v3StreamVersion, tip, strings.ToLower(tipHash.Hex())).Scan(&projection)
		expected, e := v3ExpectedProjection(totals)
		if e != nil || r.Error != nil || r.RowsAffected != 1 || !v3ProjectionEqual(expected, projection) {
			return zero, fmt.Errorf("%w: economic parity mismatch", errV3Stream)
		}
	}
	if e = auditV3CoveredPolicy(ctx, tx, rpc, s, b, policyLogs, tip, tipHash, state); e != nil {
		return zero, e
	}
	if e = v3StreamAnchor(ctx, rpc, s, tip, tipHash); e != nil {
		return zero, e
	}
	if e = v3StreamAnchor(ctx, rpc, s, stream.CursorBlock, common.HexToHash(stream.CursorHash)); e != nil {
		return zero, e
	}
	return totals, nil
}

func v3EffectsEqual(a, b []v3ParityEffect) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[v3ParityEffect]int{}
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func auditV3CoveredPolicy(ctx context.Context, tx *gorm.DB, rpc v3ScannerRPC, s v3StreamScope, b idov3whitelist.ExpectedProjectBinding, logs []types.Log, tip uint64, hash common.Hash, state string) error {
	if !v3PolicyJournalEnabled() {
		if len(logs) > 0 {
			return errV3Stream
		}
		return nil
	}
	var rows []struct {
		BlockNumber       uint64
		BlockHash, TxHash string
		LogIndex          uint
		Version           uint64
		Enabled           bool
		Root              string
	}
	if tx.Raw(`SELECT block_number,block_hash,tx_hash,log_index,version,enabled,root FROM ido_v3_policy_event_journal WHERE chain_id=? AND proxy_address=? AND project_id=? AND state=? ORDER BY block_number,log_index`, s.Chain, v3Lower(s.Proxy), b.ProjectID, state).Scan(&rows).Error != nil || len(rows) != len(logs) {
		return errV3Stream
	}
	for i, l := range logs {
		event, e := commonchain.DecodeV3PolicyLog(l, s.Chain, s.Proxy)
		if e != nil {
			return e
		}
		row := rows[i]
		if row.BlockNumber != l.BlockNumber || row.BlockHash != strings.ToLower(l.BlockHash.Hex()) || row.TxHash != strings.ToLower(l.TxHash.Hex()) || row.LogIndex != l.Index || row.Version != event.Version || row.Enabled != event.Enabled || row.Root != strings.ToLower(event.Root.Hex()) {
			return errV3Stream
		}
		receipt, e := rpc.TransactionReceipt(ctx, l.TxHash)
		if e != nil || commonchain.MatchV3PolicyEvent(event, s.Chain, s.Proxy, b.ProjectID, event.Version, event.Enabled, event.Root, receipt) != nil {
			return errV3Stream
		}
		// If an Admin Safe observation exists it must still match the original
		// receipt and immutable distinct-actor decision. Missing observations
		// remain unauthorizing chain facts, not fabricated approvals.
		var executions []struct {
			SafeTxHash, ReceiptHash string
			SnapshotDigest          string
			ReceiptBlock            uint64
			Initiator, Approver     uint64
			Decision                string
			Enabled                 bool
			Root                    string
			ExpectedVersion         uint64
		}
		if tx.Raw(`SELECT e.safe_tx_hash,e.receipt_hash,e.receipt_block,COALESCE(r.snapshot_digest,'') AS snapshot_digest,r.initiator_admin_id AS initiator,d.approver_admin_id AS approver,d.decision,r.enabled,r.root,r.expected_version FROM ido_v3_policy_review_execution e JOIN ido_v3_policy_review r ON r.id=e.review_id JOIN ido_v3_policy_review_decision d ON d.review_id=r.id WHERE r.chain_id=? AND r.proxy_address=? AND r.project_id=? AND r.target_version=? AND e.tx_hash=?`, s.Chain, v3Lower(s.Proxy), b.ProjectID, event.Version, strings.ToLower(l.TxHash.Hex())).Scan(&executions).Error != nil || len(executions) > 1 {
			return errV3Stream
		}
		if len(executions) == 1 {
			x := executions[0]
			if x.Initiator == 0 || x.Approver == 0 || x.Initiator == x.Approver || x.Decision != "approved" || x.ReceiptBlock != l.BlockNumber || !strings.EqualFold(x.ReceiptHash, l.BlockHash.Hex()) || x.Enabled != event.Enabled || !strings.EqualFold(x.Root, event.Root.Hex()) {
				return errV3Stream
			}
			safe, ok := rpc.(idov3whitelist.SafeExecutionRPC)
			if !ok {
				return errV3Stream
			}
			change := idov3whitelist.PolicyChange{ExpectedVersion: x.ExpectedVersion, TargetVersion: event.Version, Enabled: event.Enabled, Root: event.Root}
			if change.Enabled {
				change.Candidate = &idov3whitelist.SnapshotCommitment{Version: event.Version, Root: event.Root, Digest: common.HexToHash(x.SnapshotDigest)}
			}
			if idov3whitelist.CheckSafePolicyExecution(ctx, safe, idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(s.Chain), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: s.Proxy}, common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef"), change, l.TxHash, common.HexToHash(x.SafeTxHash), receipt) != nil {
				return errV3Stream
			}
		}
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(s.Chain), ProjectID: new(big.Int).SetUint64(b.ProjectID), ProxyAddress: s.Proxy}
	observed, e := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, s.Runtime, tip, hash)
	if e != nil {
		return e
	}
	if len(logs) == 0 {
		if observed.Policy.Version != 0 {
			return errV3Stream
		}
	} else {
		last, e := commonchain.DecodeV3PolicyLog(logs[len(logs)-1], s.Chain, s.Proxy)
		if e != nil || observed.Policy.Version != last.Version || observed.Policy.Root != last.Root || observed.Policy.Enabled != last.Enabled {
			return errV3Stream
		}
	}
	return nil
}
