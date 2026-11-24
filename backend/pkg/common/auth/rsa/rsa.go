package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
)

func LoadPublicKey(publicKey string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKey))
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func LoadPrivateKey(privateKey string) (*rsa.PrivateKey, error) {
	blocks, _ := pem.Decode([]byte(privateKey))
	key, err := x509.ParsePKCS1PrivateKey(blocks.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func Encrypt(publicKey *rsa.PublicKey, data []byte) ([]byte, error) {
	text, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, data, nil)
	if err != nil {
		return nil, err
	}
	base64text := base64.StdEncoding.EncodeToString(text)
	return []byte(base64text), nil
}

func Decrypt(privateKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	text, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, err
	}
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, text, nil)
}
