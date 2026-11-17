package resourceaccess

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

func newOpaqueSecret() (string, string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(digest[:]), nil
}

func newOpaqueID(prefix string) (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(entropy[:]), nil
}

func uintString(value uint) string { return strconv.FormatUint(uint64(value), 10) }
