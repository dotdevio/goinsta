package goinsta

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"time"
)

func RSADecodePublicKeyFromBase64(pubKeyBase64 string) (*rsa.PublicKey, error) {
	pubKey, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	block, _ := pem.Decode(pubKey)
	pKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return pKey.(*rsa.PublicKey), nil
}

func AESGCMEncrypt(key, data, additionalData []byte) (iv, encrypted, tag []byte, err error) {
	iv = make([]byte, 12)
	rand.Read(iv)

	var block cipher.Block
	block, err = aes.NewCipher(key)
	if err != nil {
		err = errors.New(fmt.Sprintf("error when creating cipher: %s", err))
		fmt.Println(err)
		return
	}

	var aesgcm cipher.AEAD
	aesgcm, err = cipher.NewGCM(block)
	if err != nil {
		err = errors.New(fmt.Sprintf("error when creating gcm: %s", err))
		fmt.Println(err)
		return
	}

	encrypted = aesgcm.Seal(nil, iv, data, additionalData)
	tag = encrypted[len(encrypted)-16:]       // Extracting last 16 bytes authentication tag
	encrypted = encrypted[:len(encrypted)-16] // Extracting raw Encrypted data without IV & Tag for use in NodeJS

	return
}

func RSAPublicKeyPKCS1Encrypt(publicKey *rsa.PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, publicKey, data)
}

func EncryptPassword(password, pubKeyEncoded string, pubKeyVersion int, t string) (string, error) {
	if t == "" {
		t = strconv.Itoa(int(time.Now().Unix()))
	}

	// Get the public key
	publicKey, err := RSADecodePublicKeyFromBase64(pubKeyEncoded)
	if err != nil {
		return "", err
	}

	// Data to be encrypted by RSA PKCS1
	randKey := make([]byte, 32)
	rand.Read(randKey)

	// Encrypt the random key that will be used to encrypt the password
	randKeyEncrypted, err := RSAPublicKeyPKCS1Encrypt(publicKey, randKey)
	if err != nil {
		return "", err
	}

	// Get the size of the encrypted random key
	randKeyEncryptedSize := make([]byte, 2)
	binary.LittleEndian.PutUint16(randKeyEncryptedSize[:], uint16(len(randKeyEncrypted)))

	// Encrypt the password using AES GCM with the random key
	iv, encrypted, tag, err := AESGCMEncrypt(randKey, []byte(password), []byte(t))
	if err != nil {
		return "", err
	}

	// Combine the parts
	s := []byte{}
	prefix := []byte{1, byte(pubKeyVersion)}
	parts := [][]byte{prefix, iv, randKeyEncryptedSize, randKeyEncrypted, tag, encrypted}
	for _, b := range parts {
		s = append(s, b...)
	}

	encCrypt := base64.StdEncoding.EncodeToString(s)

	return encCrypt, nil
}