package chain

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/idov3whitelist"
)

// DecodeV3ReplayLog converts a log from an explicitly supplied V3 source into
// one replay input. The caller must authenticate the source and receipt,
// establish finality and persist the result; this function does none of them.
// In particular, V2 and V3 share event topics, so no topic-only routing.
func DecodeV3ReplayLog(log types.Log, binding idov3whitelist.ExpectedProjectBinding) (V3ReplayEvent, error) {
	var empty V3ReplayEvent
	if binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		binding.PaymentToken == binding.ProjectToken || log.Address != binding.ProxyAddress ||
		log.Removed || len(log.Topics) == 0 {
		return empty, errors.New("V3 replay log source invalid")
	}
	if log.Topics[0] == v3CreatedTopic {
		created, err := DecodeV3CreatedLog(log, binding.ChainID, binding.ProxyAddress)
		if err != nil || MatchV3CreatedBinding(created, binding) != nil {
			return empty, errors.New("V3 replay creation log invalid")
		}
		return V3ReplayEvent{Created: created}, nil
	}
	if log.Topics[0] == v3PurchasedTopic {
		purchase, err := DecodeV3PurchaseLog(log, binding.ChainID, binding.ProxyAddress)
		if err != nil || purchase.ProjectID != binding.ProjectID || purchase.PaymentToken != binding.PaymentToken || purchase.ProjectToken != binding.ProjectToken {
			return empty, errors.New("V3 replay purchase log invalid")
		}
		return V3ReplayEvent{Purchase: purchase}, nil
	}
	if _, known := v3LifecycleShapes[log.Topics[0]]; known {
		lifecycle, err := DecodeV3LifecycleLog(log, binding.ChainID, binding.ProxyAddress)
		if err != nil || MatchV3LifecycleBinding(lifecycle, binding) != nil {
			return empty, errors.New("V3 replay lifecycle log invalid")
		}
		return V3ReplayEvent{Lifecycle: lifecycle}, nil
	}
	return empty, errors.New("V3 replay topic unsupported")
}
