package settings

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

var ErrInvalidTwitterVerificationSetting = errors.New("invalid twitter verification setting")

// TwitterVerificationDisabled reads the policy directly from MySQL so an
// administrator's change takes effect immediately. Missing configuration is
// deliberately strict (false). Invalid configuration is returned as an error
// so callers can fail closed and surface the operational problem.
func TwitterVerificationDisabled(database *gorm.DB) (bool, error) {
	var setting entity.Settings
	err := database.Select("value").
		Where("`key` = ?", constant.TwitterVerificationDisabledSettingKey).
		Take(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	switch strings.ToLower(strings.TrimSpace(setting.Value)) {
	case "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrInvalidTwitterVerificationSetting, setting.Value)
	}
}
