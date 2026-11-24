package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/idov3whitelist"
)

// V3ReplayEvent is deliberately not a database fact: its source must first be
// authenticated against a trusted runtime, canonical receipts and a durable
// scanner. RebuildV3Replay never writes state or grants an entitlement.
type V3ReplayEvent struct {
	Created   *V3CreatedEvent
	Purchase  *V3PurchaseEvent
	Lifecycle *V3LifecycleEvent
}

// V3ReplayTotals is an offline consistency check on a complete, ordered log
// stream. It is not a balance, a wallet position or a financial ledger.
type V3ReplayTotals struct {
	State                   idov3whitelist.ProjectState
	Raised, Sold, Deposited *big.Int
	Refunded, Claimed       *big.Int
	ProceedsPaid, FeePaid   *big.Int
	ProjectTokensWithdrawn  *big.Int
	Wallets                 map[common.Address]V3ReplayWallet
}

type V3ReplayWallet struct {
	Payment, Tokens, Refunded, Claimed *big.Int
}

func RebuildV3Replay(binding idov3whitelist.ExpectedProjectBinding, entries []V3ReplayEvent) (V3ReplayTotals, error) {
	var empty V3ReplayTotals
	if binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) || binding.ProjectToken == binding.PaymentToken || len(entries) == 0 {
		return empty, errors.New("V3 replay identity or event stream invalid")
	}
	result := V3ReplayTotals{State: idov3whitelist.ProjectPending, Wallets: make(map[common.Address]V3ReplayWallet)}
	result.Raised, result.Sold, result.Deposited = new(big.Int), new(big.Int), new(big.Int)
	result.Refunded, result.Claimed = new(big.Int), new(big.Int)
	result.ProceedsPaid, result.FeePaid = new(big.Int), new(big.Int)
	result.ProjectTokensWithdrawn = new(big.Int)
	var created bool
	var previousBlock uint64
	var previousIndex uint
	var previousHash common.Hash
	var hard, soft, price, maxPerUser, scale *big.Int
	var projectOwner, feeReceiver common.Address
	var fee uint16
	var proceedsDone, feeDone, tokensDone bool
	seen := make(map[[2]uint64]struct{}, len(entries))
	parse := func(raw string) (*big.Int, error) {
		value, ok := new(big.Int).SetString(raw, 10)
		if !ok || value.Sign() < 0 || value.BitLen() > 256 || value.String() != raw {
			return nil, errors.New("V3 replay raw amount invalid")
		}
		return value, nil
	}
	for _, entry := range entries {
		var height, project uint64
		var index uint
		var hash, tx common.Hash
		if n := boolInt(entry.Created != nil) + boolInt(entry.Purchase != nil) + boolInt(entry.Lifecycle != nil); n != 1 {
			return empty, errors.New("V3 replay event kind ambiguous")
		}
		switch {
		case entry.Created != nil:
			v := entry.Created
			if MatchV3CreatedBinding(v, binding) != nil || created {
				return empty, errors.New("V3 replay creation missing or duplicated")
			}
			height, project, index, hash, tx = v.BlockNumber, v.ProjectID, v.LogIndex, v.BlockHash, v.TxHash
		case entry.Purchase != nil:
			v := entry.Purchase
			if v.ChainID != binding.ChainID || v.ProxyAddress != binding.ProxyAddress || v.ProjectID != binding.ProjectID ||
				v.PaymentToken != binding.PaymentToken || v.ProjectToken != binding.ProjectToken || v.Buyer == (common.Address{}) || v.Buyer == binding.ProxyAddress {
				return empty, errors.New("V3 replay purchase identity mismatched")
			}
			height, project, index, hash, tx = v.BlockNumber, v.ProjectID, v.LogIndex, v.BlockHash, v.TxHash
		case entry.Lifecycle != nil:
			v := entry.Lifecycle
			if MatchV3LifecycleBinding(v, binding) != nil {
				return empty, errors.New("V3 replay lifecycle identity mismatched")
			}
			height, project, index, hash, tx = v.BlockNumber, v.ProjectID, v.LogIndex, v.BlockHash, v.TxHash
		}
		if height == 0 || project != binding.ProjectID || hash == (common.Hash{}) || tx == (common.Hash{}) ||
			(previousBlock > height) || (previousBlock == height && index <= previousIndex) ||
			(previousBlock == height && hash != previousHash) {
			return empty, errors.New("V3 replay log order or block identity invalid")
		}
		key := [2]uint64{height, uint64(index)}
		if _, exists := seen[key]; exists {
			return empty, errors.New("V3 replay log repeated")
		}
		seen[key] = struct{}{}
		previousBlock, previousIndex, previousHash = height, index, hash
		if entry.Created != nil {
			created = true
			var err error
			hard, err = parse(entry.Created.HardCapRaw)
			if err != nil {
				return empty, err
			}
			soft, err = parse(entry.Created.SoftCapRaw)
			if err != nil {
				return empty, err
			}
			if hard.Sign() == 0 || soft.Sign() == 0 || soft.Cmp(hard) > 0 || entry.Created.FeeRate > 1000 {
				return empty, errors.New("V3 replay creation parameters invalid")
			}
			price, err = parse(entry.Created.TokenPriceRaw)
			if err != nil {
				return empty, err
			}
			maxPerUser, err = parse(entry.Created.MaxPerUserRaw)
			if err != nil {
				return empty, err
			}
			if price.Sign() == 0 || maxPerUser.Sign() == 0 || maxPerUser.Cmp(hard) > 0 || entry.Created.ProjectDecimals == 0 || entry.Created.ProjectDecimals > 18 || entry.Created.PaymentDecimals == 0 || entry.Created.PaymentDecimals > 18 || entry.Created.ProjectOwner == (common.Address{}) || entry.Created.FeeReceiver == (common.Address{}) {
				return empty, errors.New("V3 replay creation economics invalid")
			}
			scale = new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(entry.Created.ProjectDecimals)), nil)
			projectOwner, feeReceiver = entry.Created.ProjectOwner, entry.Created.FeeReceiver
			fee = entry.Created.FeeRate
			continue
		}
		if !created {
			return empty, errors.New("V3 replay lifecycle before creation")
		}
		if entry.Purchase != nil {
			if result.State != idov3whitelist.ProjectActive {
				return empty, errors.New("V3 replay purchase outside active state")
			}
			payment, err := parse(entry.Purchase.PaymentRaw)
			if err != nil {
				return empty, err
			}
			sold, err := parse(entry.Purchase.ProjectTokenRaw)
			if err != nil {
				return empty, err
			}
			if payment.Sign() == 0 || sold.Sign() == 0 {
				return empty, errors.New("V3 replay purchase zero amount")
			}
			if sold.Cmp(new(big.Int).Div(new(big.Int).Mul(payment, scale), price)) != 0 {
				return empty, errors.New("V3 replay purchased tokens disagree with price")
			}
			wallet := result.Wallets[entry.Purchase.Buyer]
			if wallet.Payment == nil {
				wallet = newV3ReplayWallet()
			}
			wallet.Payment.Add(wallet.Payment, payment)
			wallet.Tokens.Add(wallet.Tokens, sold)
			if wallet.Payment.Cmp(maxPerUser) > 0 || wallet.Tokens.BitLen() > 256 {
				return empty, errors.New("V3 replay wallet purchase exceeds limit")
			}
			result.Wallets[entry.Purchase.Buyer] = wallet
			result.Raised.Add(result.Raised, payment)
			result.Sold.Add(result.Sold, sold)
			if result.Raised.Cmp(hard) > 0 || result.Sold.Cmp(result.Deposited) > 0 || result.Sold.BitLen() > 256 {
				return empty, errors.New("V3 replay purchase exceeds reserve or hard cap")
			}
			continue
		}
		v := entry.Lifecycle
		value, err := parse(v.AmountRaw)
		if v.Kind == "ProjectActivated" {
			value = new(big.Int)
			err = nil
		}
		if err != nil {
			return empty, err
		}
		switch v.Kind {
		case "ProjectTokensDeposited":
			if result.State != idov3whitelist.ProjectPending || value.Sign() == 0 {
				return empty, errors.New("V3 replay deposit state invalid")
			}
			result.Deposited.Add(result.Deposited, value)
			if result.Deposited.BitLen() > 256 {
				return empty, errors.New("V3 replay deposit overflows uint256")
			}
		case "ProjectActivated":
			if result.State != idov3whitelist.ProjectPending {
				return empty, errors.New("V3 replay duplicate activation")
			}
			// JoinCareIDOv3.activateProject requires deposited project tokens
			// to cover the soft-cap purchase at the creation-time token price.
			// Match Solidity Math.mulDiv's integer floor, not a rounded quote.
			required := new(big.Int).Div(new(big.Int).Mul(soft, scale), price)
			if result.Deposited.Cmp(required) < 0 {
				return empty, errors.New("V3 replay activation below soft-cap token reserve")
			}
			result.State = idov3whitelist.ProjectActive
		case "ProjectEnded":
			if result.State != idov3whitelist.ProjectActive || value.Cmp(result.Raised) != 0 ||
				(v.Status != 2 && v.Status != 3) || (v.Status == 2) != (result.Raised.Cmp(soft) >= 0) {
				return empty, errors.New("V3 replay settlement inconsistent")
			}
			result.State = idov3whitelist.ProjectState(v.Status)
		case "ProjectCancelled":
			if result.State != idov3whitelist.ProjectActive && result.State != idov3whitelist.ProjectPending || value.Cmp(result.Raised) != 0 {
				return empty, errors.New("V3 replay cancellation inconsistent")
			}
			result.State = idov3whitelist.ProjectCancelled
		case "Refunded":
			if result.State != idov3whitelist.ProjectFailed && result.State != idov3whitelist.ProjectCancelled || value.Sign() == 0 {
				return empty, errors.New("V3 replay refund state invalid")
			}
			wallet, ok := result.Wallets[v.Recipient]
			if !ok || wallet.Refunded.Sign() != 0 || value.Cmp(wallet.Payment) != 0 {
				return empty, errors.New("V3 replay refund beneficiary or amount invalid")
			}
			wallet.Refunded.Add(wallet.Refunded, value)
			result.Wallets[v.Recipient] = wallet
			result.Refunded.Add(result.Refunded, value)
			if result.Refunded.Cmp(result.Raised) > 0 {
				return empty, errors.New("V3 replay refund exceeds raised")
			}
		case "Claimed":
			if result.State != idov3whitelist.ProjectSuccessful || value.Sign() == 0 {
				return empty, errors.New("V3 replay claim state invalid")
			}
			wallet, ok := result.Wallets[v.Recipient]
			if !ok {
				return empty, errors.New("V3 replay claim beneficiary invalid")
			}
			wallet.Claimed.Add(wallet.Claimed, value)
			if wallet.Claimed.Cmp(wallet.Tokens) > 0 {
				return empty, errors.New("V3 replay wallet claim exceeds purchase")
			}
			result.Wallets[v.Recipient] = wallet
			result.Claimed.Add(result.Claimed, value)
			if result.Claimed.Cmp(result.Sold) > 0 {
				return empty, errors.New("V3 replay claim exceeds sold")
			}
		case "ProjectProceedsWithdrawn":
			platform, err := parse(v.SecondaryRaw)
			if err != nil {
				return empty, err
			}
			if result.State != idov3whitelist.ProjectSuccessful || proceedsDone || v.Recipient != projectOwner ||
				new(big.Int).Add(value, platform).Cmp(result.Raised) != 0 ||
				platform.Cmp(new(big.Int).Div(new(big.Int).Mul(result.Raised, big.NewInt(int64(fee))), big.NewInt(10000))) != 0 {
				return empty, errors.New("V3 replay proceeds inconsistent")
			}
			result.ProceedsPaid.Set(value)
			proceedsDone = true
		case "PlatformFeeWithdrawn":
			if result.State != idov3whitelist.ProjectSuccessful || feeDone || v.Recipient != feeReceiver ||
				value.Cmp(new(big.Int).Div(new(big.Int).Mul(result.Raised, big.NewInt(int64(fee))), big.NewInt(10000))) != 0 {
				return empty, errors.New("V3 replay platform fee inconsistent")
			}
			result.FeePaid.Set(value)
			feeDone = true
		case "ProjectTokensWithdrawn":
			if (result.State != idov3whitelist.ProjectSuccessful && result.State != idov3whitelist.ProjectFailed && result.State != idov3whitelist.ProjectCancelled) || tokensDone || v.Recipient != projectOwner {
				return empty, errors.New("V3 replay project token exit invalid")
			}
			expected := new(big.Int).Set(result.Deposited)
			if result.State == idov3whitelist.ProjectSuccessful {
				expected.Sub(expected, result.Sold)
			}
			if value.Cmp(expected) != 0 {
				return empty, errors.New("V3 replay project token exit mismatch")
			}
			result.ProjectTokensWithdrawn.Set(value)
			tokensDone = true
		default:
			return empty, errors.New("V3 replay lifecycle unsupported")
		}
	}
	return result, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func newV3ReplayWallet() V3ReplayWallet {
	return V3ReplayWallet{new(big.Int), new(big.Int), new(big.Int), new(big.Int)}
}
