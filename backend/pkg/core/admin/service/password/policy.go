// Package password 密码策略（设计 §3.8.3：14 位 + 大小写数字符号 + 90 天轮换）
package password

import (
	"errors"
	"unicode"
)

const MinLength = 14

// ErrTooShort 等错误暴露给上层封装成用户友好文案
var (
	ErrTooShort    = errors.New("password must be at least 14 characters")
	ErrNoUpper     = errors.New("password must contain at least one uppercase letter")
	ErrNoLower     = errors.New("password must contain at least one lowercase letter")
	ErrNoDigit     = errors.New("password must contain at least one digit")
	ErrNoSymbol    = errors.New("password must contain at least one symbol (!@#... etc)")
)

// Check 强度校验：14 位 + 4 类字符全都要
func Check(pwd string) error {
	if len(pwd) < MinLength {
		return ErrTooShort
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range pwd {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || (r <= 0x7F && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' '):
			hasSymbol = true
		}
	}
	if !hasUpper {
		return ErrNoUpper
	}
	if !hasLower {
		return ErrNoLower
	}
	if !hasDigit {
		return ErrNoDigit
	}
	if !hasSymbol {
		return ErrNoSymbol
	}
	return nil
}

// DaysSinceChanged 用于 90 天轮换提示；返回 -1 表示未记录
func DaysSinceChanged(secondsAgo int64) int {
	if secondsAgo < 0 {
		return -1
	}
	return int(secondsAgo / 86400)
}

// RotateThresholdDays 建议轮换阈值
const RotateThresholdDays = 90
