package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"os"
)

var aesKey []byte

func init() {
	key := os.Getenv("AES_KEY")
	if key == "" {
		key = "1234567890123456" // 仅本地开发备用
	}
	aesKey = []byte(key)
}

// ===================== 加密 =====================
func Encrypt(plainText string) (string, error) {
	plainBytes := []byte(plainText)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	blockSize := block.BlockSize()
	plainBytes = pkcs7Padding(plainBytes, blockSize)

	mode := cipher.NewCBCEncrypter(block, aesKey[:blockSize])
	crypted := make([]byte, len(plainBytes))
	mode.CryptBlocks(crypted, plainBytes)

	return base64.StdEncoding.EncodeToString(crypted), nil
}

// ===================== 解密 =====================
func Decrypt(cryptedText string) (string, error) {
	cryptedBytes, err := base64.StdEncoding.DecodeString(cryptedText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	blockSize := block.BlockSize()
	if len(cryptedBytes) < blockSize {
		return "", errors.New("cipher too short")
	}

	mode := cipher.NewCBCDecrypter(block, aesKey[:blockSize])
	origData := make([]byte, len(cryptedBytes))
	mode.CryptBlocks(origData, cryptedBytes)
	origData = pkcs7UnPadding(origData)

	return string(origData), nil
}

// ===================== 填充 =====================
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

func pkcs7UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}
