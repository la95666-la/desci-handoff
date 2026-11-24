package chainevent

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"regexp"

	"joincare_server/pkg/common/idov3whitelist"
)

// This fake models fixed-block read responses, not an approved source. It
// deliberately has no signing or broadcasting method.
type v3PinnedScanStub struct {
	chain                      uint64
	head                       *types.Header
	parentHead                 *types.Header
	changeFinalParent          bool
	parentReads                int
	proxy, impl, payment, sale common.Address
	project                    uint64
	tuple                      []byte
	projectReads               int
	changeProjectAfter         int
	logs                       []types.Log
	filters                    int
	changeSecondFilter         bool
	changeThirdFilter          bool
	changeNthFilter            int
	thirdFilterLogs            []types.Log
	receipt                    *types.Receipt
	receipts                   map[common.Hash]*types.Receipt
	changeReceipt              bool
	changeThirdReceipt         bool
	changeNthReceipt           int
	receiptReads               int
	// Test-only end-of-block policy for M3 same-case governance scans.
	policy                  []byte
	owner, governor         common.Address
	ownerCode, governorCode []byte
	threshold               uint64
}

func (s *v3PinnedScanStub) ChainID(context.Context) (*big.Int, error) {
	return new(big.Int).SetUint64(s.chain), nil
}
func (s *v3PinnedScanStub) BlockNumber(context.Context) (uint64, error) {
	return s.head.Number.Uint64() + 3, nil
}
func (s *v3PinnedScanStub) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if s.parentHead != nil && n.Cmp(s.parentHead.Number) == 0 {
		s.parentReads++
		if s.changeFinalParent && s.parentReads > 1 {
			changed := *s.parentHead
			changed.Extra = []byte("replaced parent")
			return &changed, nil
		}
		return s.parentHead, nil
	}
	if n.Cmp(s.head.Number) != 0 {
		return nil, errors.New("unexpected height")
	}
	return s.head, nil
}
func (s *v3PinnedScanStub) CodeAt(_ context.Context, a common.Address, n *big.Int) ([]byte, error) {
	if n.Cmp(s.head.Number) != 0 {
		return nil, errors.New("wrong height")
	}
	if a == s.proxy {
		return []byte{0x60, 0x80}, nil
	}
	if a == s.impl {
		return []byte{0x60, 0x01}, nil
	}
	if a == s.owner && s.ownerCode != nil {
		return s.ownerCode, nil
	}
	if a == s.governor && s.governorCode != nil {
		return s.governorCode, nil
	}
	return nil, errors.New("wrong code source")
}
func (s *v3PinnedScanStub) StorageAt(_ context.Context, a common.Address, k common.Hash, n *big.Int) ([]byte, error) {
	if a != s.proxy || n.Cmp(s.head.Number) != 0 || k != common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc") {
		return nil, errors.New("wrong slot")
	}
	return common.LeftPadBytes(s.impl.Bytes(), 32), nil
}
func (s *v3PinnedScanStub) CallContract(_ context.Context, c ethereum.CallMsg, n *big.Int) ([]byte, error) {
	if c.To != nil && *c.To == s.governor && s.governorCode != nil && len(c.Data) == 4 && bytes.Equal(c.Data, crypto.Keccak256([]byte("getThreshold()"))[:4]) {
		return new(big.Int).SetUint64(s.threshold).FillBytes(make([]byte, 32)), nil
	}
	if c.To == nil || *c.To != s.proxy || n.Cmp(s.head.Number) != 0 || len(c.Data) < 4 {
		return nil, errors.New("wrong call")
	}
	switch {
	case bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("contractVersion()"))[:4]):
		h := crypto.Keccak256Hash([]byte("joincare.ido_v3_stablecoin.v1"))
		return h[:], nil
	case bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("getWhitelistPolicy(uint256)"))[:4]) && len(c.Data) == 36 && new(big.Int).SetBytes(c.Data[4:]).Uint64() == s.project:
		if s.policy != nil {
			return s.policy, nil
		}
		return make([]byte, 96), nil
	case len(c.Data) == 4 && bytes.Equal(c.Data, crypto.Keccak256([]byte("owner()"))[:4]) && s.owner != (common.Address{}):
		return common.LeftPadBytes(s.owner.Bytes(), 32), nil
	case len(c.Data) == 4 && bytes.Equal(c.Data, crypto.Keccak256([]byte("whitelistGovernor()"))[:4]) && s.governor != (common.Address{}):
		return common.LeftPadBytes(s.governor.Bytes(), 32), nil
	case len(c.Data) == 4 && bytes.Equal(c.Data, crypto.Keccak256([]byte("operator()"))[:4]) && s.governor != (common.Address{}):
		return common.LeftPadBytes(s.governor.Bytes(), 32), nil
	case len(c.Data) == 132 && bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4]) && s.governor != (common.Address{}) && c.From == s.governor:
		return nil, nil
	case bytes.Equal(c.Data[:4], crypto.Keccak256([]byte("getProject(uint256)"))[:4]) && len(c.Data) == 36 && new(big.Int).SetBytes(c.Data[4:]).Uint64() == s.project:
		s.projectReads++
		if s.changeProjectAfter > 0 && s.projectReads >= s.changeProjectAfter {
			changed := append([]byte(nil), s.tuple...)
			changed[32+9*32+31]++ // immutable token price in the ABI tuple
			return changed, nil
		}
		return s.tuple, nil
	}
	return nil, errors.New("unexpected selector")
}
func (s *v3PinnedScanStub) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	s.receiptReads++
	if receipt, ok := s.receipts[hash]; ok {
		return receipt, nil
	}
	if s.receipt == nil || hash != s.receipt.TxHash {
		return nil, errors.New("no transaction")
	}
	if s.changeReceipt && s.receiptReads == 2 {
		changed := *s.receipt
		changed.Status = types.ReceiptStatusFailed
		return &changed, nil
	}
	if (s.changeThirdReceipt && s.receiptReads == 3) || (s.changeNthReceipt > 0 && s.receiptReads == s.changeNthReceipt) {
		changed := *s.receipt
		changed.Status = types.ReceiptStatusFailed
		return &changed, nil
	}
	return s.receipt, nil
}
func (s *v3PinnedScanStub) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	s.filters++
	if q.FromBlock.Cmp(s.head.Number) != 0 || q.ToBlock.Cmp(s.head.Number) != 0 || len(q.Addresses) != 1 || q.Addresses[0] != s.proxy {
		return nil, errors.New("wrong filter")
	}
	if s.changeSecondFilter && s.filters == 2 {
		return []types.Log{{Address: s.proxy, Index: 1}}, nil
	}
	if (s.changeThirdFilter && s.filters == 3) || (s.changeNthFilter > 0 && s.filters == s.changeNthFilter) {
		if s.thirdFilterLogs != nil {
			return append([]types.Log(nil), s.thirdFilterLogs...), nil
		}
		return []types.Log{{Address: s.proxy, Index: 1}}, nil
	}
	return append([]types.Log(nil), s.logs...), nil
}

func v3PendingTuple(project uint64, sale, payment common.Address) []byte {
	// getProject returns one dynamic tuple: offset 32, 23 head words and
	// two one-element arrays with no gaps or trailing bytes.
	result := make([]byte, 32+23*32+2*64)
	set := func(i int, v *big.Int) { copy(result[32+i*32:32+(i+1)*32], v.FillBytes(make([]byte, 32))) }
	set(0, big.NewInt(0).SetUint64(project))
	for i, a := range map[int]common.Address{1: sale, 2: payment, 3: sale, 4: payment} {
		set(i, big.NewInt(0).SetBytes(a.Bytes()))
	}
	for i, v := range map[int]int64{5: 18, 6: 18, 7: 200, 9: 20, 10: 1000, 11: 500, 12: 900, 13: 100, 14: 200, 17: 400, 21: 23 * 32, 22: 23*32 + 64} {
		set(i, big.NewInt(v))
	}
	copy(result[:32], big.NewInt(32).FillBytes(make([]byte, 32)))
	copy(result[32+23*32:], big.NewInt(1).FillBytes(make([]byte, 32)))
	copy(result[32+23*32+32:], big.NewInt(300).FillBytes(make([]byte, 32)))
	copy(result[32+23*32+64:], big.NewInt(1).FillBytes(make([]byte, 32)))
	copy(result[32+23*32+96:], big.NewInt(10000).FillBytes(make([]byte, 32)))
	return result
}

func TestV3ScannerReadsEmptyPinnedBlockAndRejectsChangingFilter(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	stub := &v3PinnedScanStub{chain: 31337, head: &types.Header{Number: big.NewInt(90), ParentHash: common.HexToHash("0xa1")}, proxy: proxy, impl: impl, payment: payment, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, payment)}
	parent, hash, logs, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3)
	if err != nil || parent != stub.head.ParentHash || hash != stub.head.Hash() || len(logs) != 0 || stub.filters != 2 {
		t.Fatalf("empty pinned block rejected: %v", err)
	}
	stub.filters = 0
	stub.changeSecondFilter = true
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("changing filter response accepted")
	}
	stub.filters = 0
	stub.changeSecondFilter = false
	stub.chain = 56
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("wrong chain accepted")
	}
}

func TestV3CheckedScannerRejectsCursorFenceBeforeTrustedReads(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"), ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "state"}).AddRow(90, common.HexToHash("0xa1").Hex(), "reorg_halted"))
	rpc := v3ScannerEarlyRejectRPC{chain: 31337, latest: 99}
	if err := v3ScanCheckedBlock(context.Background(), db, rpc, binding, idov3whitelist.RuntimeIdentity{}, 91, 3); err == nil {
		t.Fatal("halted cursor passed into RPC/project reader")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3ScannerReadsCreatedReceiptWithoutTopicOnlyRouting(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	header := &types.Header{Number: big.NewInt(90), ParentHash: common.HexToHash("0xa1")}
	// ABI pack fields independently of the event decoder.
	uint := func(v int64) *big.Int { return big.NewInt(v) }
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	uint8Type, err := abi.NewType("uint8", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	uint16Type, err := abi.NewType("uint16", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	uint256Type, err := abi.NewType("uint256", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: addressType}, {Type: addressType}, {Type: uint8Type}, {Type: uint8Type}, {Type: uint16Type}}
	for i := 0; i < 6; i++ {
		args = append(args, abi.Argument{Type: uint256Type})
	}
	data, err := args.Pack(sale, payment, uint8(18), uint8(18), uint16(200), uint(20), uint(1000), uint(500), uint(900), uint(100), uint(200))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Address: proxy, BlockNumber: 90, BlockHash: header.Hash(), TxHash: common.HexToHash("0xb1"), Index: 3,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(sale.Bytes()), common.BytesToHash(payment.Bytes())}, Data: data}
	stub := &v3PinnedScanStub{chain: 31337, head: header, proxy: proxy, impl: impl, payment: payment, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, payment), logs: []types.Log{log}, receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: header.Hash(), TxHash: log.TxHash, Logs: []*types.Log{&log}}}
	parent, hash, selected, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3)
	if err != nil || parent != header.ParentHash || hash != header.Hash() || len(selected) != 1 || stub.filters != 2 || stub.receiptReads != 2 {
		t.Fatalf("complete created receipt rejected: %v", err)
	}
	// Exercise production's opt-in first-pass wrapper, including an already
	// retained receipt followed by a changed independent second receipt.
	stub.filters, stub.receiptReads = 0, 0
	shared := newV3TickReads(context.Background(), stub)
	defer shared.close()
	if _, _, _, err := v3ScanBlock(context.Background(), shared, binding, runtime, 90, 3); err != nil {
		t.Fatalf("shared complete receipt rejected: %v", err)
	}
	stub.receiptReads = 1 // next source read is the second-pass receipt
	stub.changeReceipt = true
	if _, _, _, err := v3ScanBlock(context.Background(), shared, binding, runtime, 90, 3); err == nil {
		t.Fatal("retained first receipt hid changed independent receipt")
	}
	stub.changeReceipt = false
	stub.filters, stub.receiptReads = 0, 0
	stub.changeReceipt = true
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("changed receipt accepted")
	}
	stub.filters, stub.receiptReads = 0, 0
	stub.changeReceipt = false
	changedTuple := append([]byte(nil), stub.tuple...)
	changedTuple[32+9*32+31]++
	stub.tuple = changedTuple
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("creation price disagreed with pinned getProject")
	}
	stub.tuple = v3PendingTuple(7, sale, payment)
	other := log
	other.Address = payment
	stub.logs = []types.Log{other}
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 90, 3); err == nil {
		t.Fatal("same-topic event from other address accepted")
	}
}

func TestV3ScannerReadsPurchaseAmongOtherProjectsAtSameProxy(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	buyer := common.HexToAddress("0x5555555555555555555555555555555555555555")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	header := &types.Header{Number: big.NewInt(91), ParentHash: common.HexToHash("0xa1")}
	tuple := v3PendingTuple(7, sale, payment)
	setWord := func(index int, number *big.Int) {
		copy(tuple[32+index*32:32+(index+1)*32], number.FillBytes(make([]byte, 32)))
	}
	setWord(8, big.NewInt(int64(idov3whitelist.ProjectActive)))
	setWord(9, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), big.NewInt(2)))
	setWord(15, big.NewInt(600))
	setWord(16, big.NewInt(300))
	sig := crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)"))
	word := func(n *big.Int) []byte { return n.FillBytes(make([]byte, 32)) }
	topics := []common.Hash{sig, common.BigToHash(big.NewInt(7)), common.BytesToHash(buyer.Bytes()), common.BytesToHash(payment.Bytes())}
	data := append(append(word(big.NewInt(600)), common.LeftPadBytes(sale.Bytes(), 32)...), word(big.NewInt(300))...)
	selected := types.Log{Address: proxy, BlockNumber: 91, BlockHash: header.Hash(), TxHash: common.HexToHash("0xb1"), Index: 3, Topics: topics, Data: data}
	other := selected
	other.Topics = append([]common.Hash(nil), topics...)
	other.Topics[1] = common.BigToHash(big.NewInt(8))
	other.Index, other.TxHash = 1, common.HexToHash("0xb2")
	stub := &v3PinnedScanStub{chain: 31337, head: header, proxy: proxy, impl: impl, payment: payment, sale: sale, project: 7, tuple: tuple,
		logs: []types.Log{selected, other}, receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(91), BlockHash: header.Hash(), TxHash: selected.TxHash, Logs: []*types.Log{&selected}}}
	parent, hash, logs, err := v3ScanBlock(context.Background(), stub, binding, runtime, 91, 3)
	if err != nil || parent != header.ParentHash || hash != header.Hash() || len(logs) != 1 || logs[0].Index != selected.Index || stub.filters != 2 || stub.receiptReads != 2 {
		t.Fatalf("bound purchase was not isolated: %v", err)
	}
	stub.filters, stub.receiptReads = 0, 0
	stub.receipt.Status = types.ReceiptStatusFailed
	if _, _, _, err := v3ScanBlock(context.Background(), stub, binding, runtime, 91, 3); err == nil {
		t.Fatal("failed purchase receipt accepted")
	}
}
