package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

const (
	DB_DRIVER = "postgres"
)

func AESEncrypt(password string, text []byte) ([]byte, error) {
	hasher := sha256.New()
	key := hasher.Sum([]byte(password))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func AESDecrypt(password string, text []byte) ([]byte, error) {
	hasher := sha256.New()
	key := hasher.Sum([]byte(password))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetenvWithDefault(envKey string, defaultVal string) string {
	envVal := os.Getenv(envKey)

	if len(envVal) == 0 {
		envVal = defaultVal
	}

	return envVal
}

func CreateSqlString(value string) sql.NullString {
	var nullValue sql.NullString
	if len(value) != 0 {
		nullValue = sql.NullString{value, true}
	}
	return nullValue
}
