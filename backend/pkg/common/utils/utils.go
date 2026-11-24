package utils

import (
	"fmt"
	"math/big"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func GetLastPathSegment(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	// 取路径部分（去掉结尾多余的斜杠）
	p := strings.TrimSuffix(u.Path, "/")

	// 取最后一个路径段
	last := path.Base(p)

	if last == "." || last == "/" || last == "" {
		return "", fmt.Errorf("no valid path segment found")
	}

	return last, nil
}

func GetFileExtension(urlStr string) string {
	// 解析URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// 获取路径部分
	filePath := parsedURL.Path

	// 使用path包获取基础文件名
	baseName := path.Base(filePath)

	// 使用filepath包获取扩展名
	ext := filepath.Ext(baseName)

	// 去除点号
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
	}

	return ext
}

func IntToBool(a int) bool {
	if a == 0 {
		return false
	}
	return true
}

func ParseDateStrings(dateStr string) ([]time.Time, error) {
	// 按逗号分割字符串
	parts := strings.Split(dateStr, ",")
	times := make([]time.Time, 0, len(parts))

	for _, part := range parts {
		// 去除可能存在的空格
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 解析日期，指定为 UTC 时区
		t, err := time.ParseInLocation("2006-01-02 15:04", part, time.UTC)
		if err != nil {
			return nil, err
		}
		// 转换为 UTC 时间
		times = append(times, t)
	}
	return times, nil
}

func ParseFloatStrings(floatStr string) ([]float64, error) {
	// 按逗号分割字符串
	parts := strings.Split(floatStr, ",")
	floats := make([]float64, 0, len(parts))
	for _, part := range parts {
		// 去除可能存在的空格
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 解析为浮点数
		num, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, err
		}
		floats = append(floats, num)
	}

	return floats, nil
}

func ParseTimeToBigInt(times []time.Time) ([]*big.Int, error) {
	res := make([]*big.Int, 0, len(times))
	for _, t := range times {
		res = append(res, big.NewInt(t.Unix()))
	}
	return res, nil
}

func ParseFloatToBigInt(floats []float64) ([]*big.Int, error) {
	res := make([]*big.Int, 0, len(floats))
	for _, i := range floats {
		res = append(res, big.NewInt(int64(i)))
	}
	return res, nil
}
