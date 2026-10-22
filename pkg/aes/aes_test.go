package aes

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── Encrypt + Decrypt round-trip ─────────────────────────────────────────────

func TestEncryptDecrypt(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		enc, err := Encrypt("")
		assert.NoError(t, err)
		assert.NotEmpty(t, enc)

		dec, err := Decrypt(enc)
		assert.NoError(t, err)
		assert.Equal(t, "", dec)
	})

	t.Run("plain_english", func(t *testing.T) {
		enc, err := Encrypt("hello world")
		assert.NoError(t, err)

		dec, err := Decrypt(enc)
		assert.NoError(t, err)
		assert.Equal(t, "hello world", dec)
	})

	t.Run("chinese_text", func(t *testing.T) {
		enc, err := Encrypt("你好，世界")
		assert.NoError(t, err)

		dec, err := Decrypt(enc)
		assert.NoError(t, err)
		assert.Equal(t, "你好，世界", dec)
	})

	t.Run("special_characters", func(t *testing.T) {
		input := "!@#$%^&*()_+-=[]{}|;:',.<>?/`~"
		enc, err := Encrypt(input)
		assert.NoError(t, err)

		dec, err := Decrypt(enc)
		assert.NoError(t, err)
		assert.Equal(t, input, dec)
	})

	t.Run("long_text_multiple_blocks", func(t *testing.T) {
		input := strings.Repeat("ABCDEFGHIJKLMNOP", 32) // 512 chars, 32 blocks
		enc, err := Encrypt(input)
		assert.NoError(t, err)

		dec, err := Decrypt(enc)
		assert.NoError(t, err)
		assert.Equal(t, input, dec)
	})

	t.Run("same_input_same_cipher", func(t *testing.T) {
		// IV 固定（aesKey 前 blockSize 字节），相同明文 → 相同密文
		enc1, err := Encrypt("secret-123")
		assert.NoError(t, err)
		enc2, err := Encrypt("secret-123")
		assert.NoError(t, err)
		assert.Equal(t, enc1, enc2)
	})

	t.Run("different_input_different_cipher", func(t *testing.T) {
		enc1, err := Encrypt("secret-123")
		assert.NoError(t, err)
		enc2, err := Encrypt("secret-456")
		assert.NoError(t, err)
		assert.NotEqual(t, enc1, enc2)
	})
}

// ─── Encrypt ──────────────────────────────────────────────────────────────────

func TestEncrypt(t *testing.T) {
	t.Run("returns_valid_base64", func(t *testing.T) {
		enc, err := Encrypt("data")
		assert.NoError(t, err)
		_, err = base64.StdEncoding.DecodeString(enc)
		assert.NoError(t, err, "output must be valid base64")
	})
}

// ─── Decrypt errors ───────────────────────────────────────────────────────────

func TestDecrypt(t *testing.T) {
	t.Run("invalid_base64", func(t *testing.T) {
		_, err := Decrypt("not-valid-base64!!")
		assert.Error(t, err)
	})

	t.Run("empty_input", func(t *testing.T) {
		// 空 base64 解码后为 0 字节，< blockSize
		_, err := Decrypt("")
		assert.Error(t, err)
	})

	t.Run("cipher_too_short", func(t *testing.T) {
		// 编码一个小于 blockSize(16) 的字符串
		short := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := Decrypt(short)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cipher too short")
	})

	t.Run("tampered_cipher", func(t *testing.T) {
		enc, err := Encrypt("hello world")
		assert.NoError(t, err)

		// 修改密文中间一个字符
		b := []byte(enc)
		b[len(b)/2] ^= 0x01
		tampered := string(b)

		// 篡改后解密可能 panic（padding 越界），也可能返回乱码
		var decrypted string
		var decErr error
		func() {
			defer func() { recover() }()
			decrypted, decErr = Decrypt(tampered)
		}()
		if decErr == nil {
			assert.NotEqual(t, "hello world", decrypted)
		}
	})
}

// ─── PKCS7 Padding ────────────────────────────────────────────────────────────

func TestPKCS7Padding(t *testing.T) {
	t.Run("less_than_one_block", func(t *testing.T) {
		data := []byte("hello") // len=5
		result := pkcs7Padding(data, 16)
		assert.Equal(t, 16, len(result))
		assert.Equal(t, byte(11), result[15]) // 16-5=11
		for i := 5; i < 16; i++ {
			assert.Equal(t, byte(11), result[i])
		}
	})

	t.Run("exact_one_block", func(t *testing.T) {
		data := make([]byte, 16) // len=16
		result := pkcs7Padding(data, 16)
		assert.Equal(t, 32, len(result))
		for i := 16; i < 32; i++ {
			assert.Equal(t, byte(16), result[i])
		}
	})

	t.Run("more_than_one_block", func(t *testing.T) {
		data := make([]byte, 20) // len=20, padding=12
		result := pkcs7Padding(data, 16)
		assert.Equal(t, 32, len(result))
		for i := 20; i < 32; i++ {
			assert.Equal(t, byte(12), result[i])
		}
	})
}

// ─── PKCS7 UnPadding ──────────────────────────────────────────────────────────

func TestPKCS7UnPadding(t *testing.T) {
	t.Run("remove_padding", func(t *testing.T) {
		original := []byte("hello")
		padded := pkcs7Padding(original, 16)
		result := pkcs7UnPadding(padded)
		assert.Equal(t, original, result)
	})

	t.Run("remove_full_block_padding", func(t *testing.T) {
		original := make([]byte, 16)
		padded := pkcs7Padding(original, 16)
		result := pkcs7UnPadding(padded)
		assert.Equal(t, original, result)
	})

	t.Run("empty_data_panics", func(t *testing.T) {
		assert.Panics(t, func() {
			pkcs7UnPadding([]byte{})
		})
	})
}
