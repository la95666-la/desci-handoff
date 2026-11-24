package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type RandomPool func() string

func CapitalLetter() string {
	return "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

func LowercaseLetter() string {
	return "abcdefghijklmnopqrstuvwxyz"
}

func Number() string {
	return "1234567890"
}

// GenerateRandomWord 生成随机字符
func GenerateRandomWord(length int, opts ...RandomPool) string {
	if len(opts) == 0 {
		return ""
	}
	characters := ""
	for _, opt := range opts {
		characters = characters + opt()
	}
	result := make([]byte, length)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < length; i++ {
		result[i] = characters[random.Intn(len(characters))]
	}
	return string(result)
}

// GenerateRandomWords 批量生成随机字符
func GenerateRandomWords(num, length int, opts ...RandomPool) []string {
	if len(opts) == 0 || num <= 0 {
		return []string{}
	}
	res := make([]string, num)
	characters := ""
	for _, opt := range opts {
		characters = characters + opt()
	}
	pool := RandomPool(func() string {
		return characters
	})
	for i := 0; i < num; i++ {
		res[i] = GenerateRandomWord(length, pool)
	}
	return res
}

func NewUUID() string {
	id := uuid.New().String()
	return id
}

func NewUUID32() string {
	id := uuid.New()
	var buf [32]byte
	UUidToString(buf[:], id)
	return string(buf[:])
}

func UUidToString(dst []byte, id uuid.UUID) {
	hex.Encode(dst, id[:4])
	hex.Encode(dst[8:12], id[4:6])
	hex.Encode(dst[12:16], id[6:8])
	hex.Encode(dst[16:20], id[8:10])
	hex.Encode(dst[20:], id[10:])
}

func OrderNo(uid string, businessCode string) string {
	// 时间戳部分
	timestamp := time.Now().Format("20060102150405")
	timestamp = timestamp[:len(timestamp)-1] // 移除最后一个字符，保留15位

	// 用户标识部分，这里简单使用用户ID的SHA1哈希值的后4位
	hasher := sha1.New()
	hasher.Write([]byte(uid))
	userHash := fmt.Sprintf("%x", hasher.Sum(nil))
	userIdentifier := userHash[len(userHash)-4:]

	// 随机数部分
	rand.Seed(time.Now().UnixNano())
	randomNumber := GenerateRandomWord(4, Number)

	return businessCode + timestamp + userIdentifier + randomNumber
}
