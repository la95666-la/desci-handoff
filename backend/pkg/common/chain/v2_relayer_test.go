package chain

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestIDOV2CreateProjectABIOrder(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(idoV2ABIJson))
	if err != nil {
		t.Fatalf("parse V2 ABI: %v", err)
	}
	projectToken := common.HexToAddress("0x1111111111111111111111111111111111111111")
	paymentToken := common.HexToAddress("0x2222222222222222222222222222222222222222")
	owner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	feeReceiver := common.HexToAddress("0x4444444444444444444444444444444444444444")
	data, err := parsed.Pack("createProject", big.NewInt(7), projectToken, paymentToken, owner, feeReceiver, uint16(200), big.NewInt(11), big.NewInt(12), big.NewInt(13), big.NewInt(14), big.NewInt(15), big.NewInt(16), []*big.Int{big.NewInt(17)}, []*big.Int{big.NewInt(10_000)})
	if err != nil {
		t.Fatalf("pack V2 createProject: %v", err)
	}
	values, err := parsed.Methods["createProject"].Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatalf("unpack V2 createProject: %v", err)
	}
	if len(values) != 14 || values[1].(common.Address) != projectToken || values[2].(common.Address) != paymentToken || values[3].(common.Address) != owner || values[4].(common.Address) != feeReceiver || values[5].(uint16) != 200 {
		t.Fatalf("unexpected V2 createProject argument order: %#v", values)
	}
	if values[6].(*big.Int).Cmp(big.NewInt(11)) != 0 || values[13].([]*big.Int)[0].Cmp(big.NewInt(10_000)) != 0 {
		t.Fatalf("unexpected V2 createProject values: %#v", values)
	}
}

func TestSenderLockSharedAcrossContractsForSameRelayer(t *testing.T) {
	chainID := big.NewInt(97)
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	if senderLock(chainID, address) != senderLock(new(big.Int).Set(chainID), address) {
		t.Fatal("same chain/relayer did not share nonce lock")
	}
	if senderLock(chainID, address) == senderLock(chainID, common.HexToAddress("0x2222222222222222222222222222222222222222")) {
		t.Fatal("different relayers shared nonce lock")
	}
}

func TestIDOV2JobEndCalldataUsesV2ABI(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(idoV2ABIJson))
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{ContractABI: parsed}
	data, err := client.PackEndProject(big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	method := parsed.Methods["endProject"]
	if len(data) != 36 || !strings.EqualFold(common.Bytes2Hex(data[:4]), common.Bytes2Hex(method.ID)) {
		t.Fatalf("wrong V2 endProject selector: %x", data)
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil || len(args) != 1 || args[0].(*big.Int).Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("wrong V2 project ID: %v, %v", args, err)
	}
}
