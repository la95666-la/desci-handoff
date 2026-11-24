package chain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func NewCheckInNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(b), nil
}

func Bytes32FromHex(value string) ([32]byte, error) {
	var res [32]byte
	bs, err := hexutil.Decode(value)
	if err != nil || len(bs) != 32 {
		return res, fmt.Errorf("invalid bytes32")
	}
	copy(res[:], bs)
	return res, nil
}

func BuildCheckInTypedData(chainID *big.Int, contract common.Address, walletAddress string, day int64, nonce string, deadline int64) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"CheckIn": []apitypes.Type{
				{Name: "user", Type: "address"},
				{Name: "day", Type: "uint256"},
				{Name: "nonce", Type: "bytes32"},
				{Name: "deadline", Type: "uint256"},
			},
		},
		PrimaryType: "CheckIn",
		Domain: apitypes.TypedDataDomain{
			ChainId:           gethmath.NewHexOrDecimal256(chainID.Int64()),
			VerifyingContract: contract.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"user":     walletAddress,
			"day":      strconv.FormatInt(day, 10),
			"nonce":    nonce,
			"deadline": strconv.FormatInt(deadline, 10),
		},
	}
}

func CheckInTypedDataPayload(typedData apitypes.TypedData) map[string]any {
	chainID := int64(0)
	if typedData.Domain.ChainId != nil {
		chainID = (*big.Int)(typedData.Domain.ChainId).Int64()
	}
	return map[string]any{
		"types":       typedData.Types,
		"primaryType": typedData.PrimaryType,
		"domain": map[string]any{
			"chainId":           chainID,
			"verifyingContract": typedData.Domain.VerifyingContract,
		},
		"message": typedData.Message,
	}
}

func VerifyCheckInSignature(typedData apitypes.TypedData, signature, expectedAddress string) ([]byte, error) {
	signatureBytes, err := hexutil.Decode(strings.TrimSpace(signature))
	if err != nil || len(signatureBytes) != crypto.SignatureLength {
		return nil, fmt.Errorf("invalid check-in signature")
	}

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, err
	}

	recoverSignature := append([]byte(nil), signatureBytes...)
	if recoverSignature[crypto.RecoveryIDOffset] >= 27 {
		recoverSignature[crypto.RecoveryIDOffset] -= 27
	}
	if recoverSignature[crypto.RecoveryIDOffset] > 1 {
		return nil, fmt.Errorf("invalid check-in signature")
	}

	pub, err := crypto.SigToPub(hash, recoverSignature)
	if err != nil {
		return nil, fmt.Errorf("invalid check-in signature")
	}
	recoveredAddress := crypto.PubkeyToAddress(*pub).Hex()
	if !strings.EqualFold(recoveredAddress, expectedAddress) {
		return nil, fmt.Errorf("invalid check-in signature")
	}
	return signatureBytes, nil
}
