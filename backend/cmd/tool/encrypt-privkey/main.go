// One-off tool: 把明文以太坊私钥用 AES-GCM (JC_TOKEN) 加密 → base64，写入 chain config 的 private 字段。
//
// 用法：
//   JC_TOKEN=joincare-secret! go run ./cmd/tool/encrypt-privkey \
//       -key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"joincare_server/pkg/common/auth/aes"
)

func main() {
	privKey := flag.String("key", "", "hex private key (0x prefix optional)")
	flag.Parse()

	token := os.Getenv("JC_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "JC_TOKEN env required (16/24/32 bytes)")
		os.Exit(1)
	}
	if *privKey == "" {
		fmt.Fprintln(os.Stderr, "-key required")
		os.Exit(1)
	}

	// backend HexToECDSA 需要不含 0x 前缀的 hex
	plaintext := strings.TrimPrefix(strings.TrimSpace(*privKey), "0x")

	ciphertext, err := aes.EncryptAESGCM([]byte(plaintext), []byte(token))
	if err != nil {
		fmt.Fprintln(os.Stderr, "encrypt failed:", err)
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(ciphertext))
}
