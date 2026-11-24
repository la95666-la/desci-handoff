package chaintx

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedOriginalCreateRejectsChangedProjectCallAndLateSignature(t *testing.T) {
	const projectID = uint64(19)
	proxy := common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e")
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	client, err := basechain.NewClient("http://127.0.0.1:1", key, proxy, big.NewInt(56))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	token := common.HexToAddress("0x2222222222222222222222222222222222222222")
	payment := common.HexToAddress("0x55d398326f99059ff775485246999027b3197955")
	owner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")
	start := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	call, err := basechain.PackV3Create(basechain.V3CreateParams{ProjectID: big.NewInt(int64(projectID)),
		ProjectToken: token, PaymentToken: payment, ProjectOwner: owner, FeeReceiver: receiver, FeeRate: 100,
		Price: big.NewInt(1), HardCap: big.NewInt(10), SoftCap: big.NewInt(5), MaxPerUser: big.NewInt(2),
		Start: big.NewInt(start.Unix()), End: big.NewInt(start.Add(time.Hour).Unix()),
		VestingTimes:    []*big.Int{big.NewInt(start.Add(2 * time.Hour).Unix())},
		VestingPercents: []*big.Int{big.NewInt(10000)}})
	if err != nil {
		t.Fatal(err)
	}
	raw, hash, err := client.SignLegacyTransaction(7, 1000000, big.NewInt(1), proxy, call)
	if err != nil {
		t.Fatal(err)
	}
	nonce, gas, slot := uint64(7), uint64(1000000), uint8(1)
	signedAt := start.Add(-time.Hour).UTC()
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: projectID,
		ProxyAddress: proxy, ProjectToken: token, PaymentToken: payment}
	id := idov3whitelist.ApprovedSetIdentity{ProjectID: projectID, ChainID: 56,
		Proxy: strings.ToLower(proxy.Hex()), ProjectToken: strings.ToLower(token.Hex()),
		PaymentToken: strings.ToLower(payment.Hex()), Operator: strings.ToLower(client.Address.Hex()),
		CreateCallHash: strings.ToLower(crypto.Keccak256Hash(call).Hex()), CreateDeadline: start.Unix()}
	row := entity.ChainTxSubmission{ID: 2, ChainID: 56, ProjectID: projectID,
		Action: "ido_v3_create", SenderAddress: client.Address.Hex(), ContractAddress: proxy.Hex(),
		CallData: call, RawTx: raw, TxHash: &hash, Nonce: &nonce, GasLimit: &gas, ValueRaw: "0",
		ActiveSlot: &slot, V3SignedAt: &signedAt}
	if err := approvedV3OriginalAttempt(row, binding, id); err != nil {
		t.Fatalf("exact signed attempt rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*entity.ChainTxSubmission, *idov3whitelist.ApprovedSetIdentity)
	}{
		{"foreign project", func(_ *entity.ChainTxSubmission, i *idov3whitelist.ApprovedSetIdentity) { i.ProjectID++ }},
		{"changed call commitment", func(_ *entity.ChainTxSubmission, i *idov3whitelist.ApprovedSetIdentity) {
			i.CreateCallHash = "0x" + strings.Repeat("a", 64)
		}},
		{"changed raw transaction", func(r *entity.ChainTxSubmission, _ *idov3whitelist.ApprovedSetIdentity) {
			r.RawTx = append([]byte(nil), r.RawTx...)
			r.RawTx[len(r.RawTx)-1] ^= 1
		}},
		{"late signature", func(r *entity.ChainTxSubmission, _ *idov3whitelist.ApprovedSetIdentity) {
			at := start.Add(time.Second)
			r.V3SignedAt = &at
		}},
		{"missing sender slot", func(r *entity.ChainTxSubmission, _ *idov3whitelist.ApprovedSetIdentity) { r.ActiveSlot = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate, identity := row, id
			tc.mutate(&candidate, &identity)
			if err := approvedV3OriginalAttempt(candidate, binding, identity); err == nil {
				t.Fatal("modified signed attempt accepted")
			}
		})
	}
}

func TestApprovedCreationAuditFailsClosedBeforeReceiptWithoutExecutedSet(t *testing.T) {
	if _, err := AuditApprovedV3Creation(nil, nil, nil, 19, 1, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("nil scope admitted")
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Never query an outbox hash or an RPC receipt when no executed approval
	// joins the project-specific immutable Set commitment.
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// A call into RPC before approval admission would panic this nil transport.
	var fake V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	if _, err := AuditApprovedV3Creation(context.Background(), database, fake, 19, 1,
		common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatalf("missing approval reached receipt RPC: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovedCreationReceiptBindsUniqueOriginalProjectEvent(t *testing.T) {
	proxy := common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e")
	token := common.HexToAddress("0x2222222222222222222222222222222222222222")
	payment := common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	owner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 56, ProjectID: 19, ProxyAddress: proxy,
		ProjectToken: token, PaymentToken: payment}
	id := idov3whitelist.ApprovedSetIdentity{ProjectID: 19, ChainID: 56,
		Proxy: strings.ToLower(proxy.Hex()), ProjectToken: strings.ToLower(token.Hex()),
		PaymentToken: strings.ToLower(payment.Hex()), ProjectOwner: strings.ToLower(owner.Hex()),
		FeeReceiver: strings.ToLower(receiver.Hex()), TokenDecimals: 18, PaymentDecimals: 18,
		FeeRate: 100, CreateDeadline: 1800000000}
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	blockHash := common.HexToHash("0x1234")
	txHash := common.HexToHash("0x5678")
	log := &types.Log{Address: proxy, BlockNumber: 128805700, BlockHash: blockHash,
		TxHash: txHash, Index: 3, Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")),
			common.BigToHash(big.NewInt(19)), common.BytesToHash(token.Bytes()), common.BytesToHash(payment.Bytes()),
		}}
	log.Data = append(log.Data, common.LeftPadBytes(owner.Bytes(), 32)...)
	log.Data = append(log.Data, common.LeftPadBytes(receiver.Bytes(), 32)...)
	for _, value := range []uint64{18, 18, 100, 1, 10, 5, 2, 1800000000, 1800003600} {
		log.Data = append(log.Data, word(value)...)
	}
	audit := V3ReceiptAudit{Hash: txHash, Block: 128805700, BlockHash: blockHash}
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: new(big.Int).SetUint64(audit.Block),
		BlockHash: blockHash, TxHash: txHash, Logs: []*types.Log{log}}
	if err := approvedV3CreatedReceipt(receipt, binding, id, audit); err != nil {
		t.Fatalf("matching original receipt rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*types.Receipt, *idov3whitelist.ApprovedSetIdentity)
	}{
		{"foreign project", func(_ *types.Receipt, identity *idov3whitelist.ApprovedSetIdentity) { identity.ProjectID++ }},
		{"changed fee", func(_ *types.Receipt, identity *idov3whitelist.ApprovedSetIdentity) { identity.FeeRate++ }},
		{"wrong hash", func(r *types.Receipt, _ *idov3whitelist.ApprovedSetIdentity) { r.TxHash = common.HexToHash("0xaa") }},
		{"missing log", func(r *types.Receipt, _ *idov3whitelist.ApprovedSetIdentity) { r.Logs = nil }},
		{"duplicate project log", func(r *types.Receipt, _ *idov3whitelist.ApprovedSetIdentity) {
			duplicate := *log
			duplicate.Index = 4
			r.Logs = append(r.Logs, &duplicate)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyReceipt := *receipt
			copyReceipt.Logs = append([]*types.Log(nil), receipt.Logs...)
			copyID := id
			tc.change(&copyReceipt, &copyID)
			if approvedV3CreatedReceipt(&copyReceipt, binding, copyID, audit) == nil {
				t.Fatal("changed event accepted")
			}
		})
	}
}
