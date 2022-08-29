package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"strings"
)

// 128-bit
func md5Key(key string) []byte {
	sum := md5.Sum([]byte(key))

	return sum[:]
}

func _PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func _PKCS7UnPadding(data []byte) []byte {
	n := len(data)
	unpadding := int(data[n-1])
	return data[:(n - unpadding)]
}

func AesEncrypto(text, key string) string {
	_key := md5Key(key)

	block, _ := aes.NewCipher(_key)
	blockSize := block.BlockSize()

	in := []byte(text)
	in = _PKCS7Padding(in, blockSize)

	blockMode := cipher.NewCBCEncrypter(block, _key[:blockSize])

	crypted := make([]byte, len(in))
	blockMode.CryptBlocks(crypted, in)

	return base64.StdEncoding.EncodeToString(crypted)
}

func AesDecrypto(crypted, key string) (data string, err error) {
	defer func() {
		_err := recover()
		if _err != nil {
			err = fmt.Errorf("AesDecrypto panic:%v", _err)
		}
	}()

	_key := md5Key(key)
	pw, err := base64.StdEncoding.DecodeString(crypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(_key)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()

	blockMode := cipher.NewCBCDecrypter(block, _key[:blockSize])

	out := make([]byte, len(pw))

	blockMode.CryptBlocks(out, pw)
	out = _PKCS7UnPadding(out)

	return string(out), nil
}

// Base64Decode decode base64 string,return username,password
// http://play.golang.org/p/CNIwzF1L6l
func Base64Decode(auth string) (username, password string, err error) {
	authb, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", err
	}
	cone := strings.Split(string(authb), ":")
	username = cone[0]
	if len(cone) > 1 {
		password = cone[1]
	}
	return username, password, err
}

// Base64Encode encode string by base64
func Base64Encode(username, password string) string {
	src := []byte(username + ":" + password)
	return base64.StdEncoding.EncodeToString(src)
}
