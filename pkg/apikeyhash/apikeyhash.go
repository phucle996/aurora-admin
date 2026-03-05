package apikeyhash

import (
	"admin/pkg/errorvar"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory  = 64 * 1024
	argon2Time    = 3
	argon2Threads = 2
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

func Hash(apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errorvar.ErrAPIKeyInvalid
	}

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(apiKey),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argon2Memory, argon2Time, argon2Threads, saltB64, hashB64), nil
}

func Compare(stored, provided string) bool {
	stored = strings.TrimSpace(stored)
	provided = strings.TrimSpace(provided)
	if stored == "" || provided == "" {
		return false
	}

	// Backward compatible with legacy plain-text storage.
	if !strings.HasPrefix(stored, "$argon2id$") {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) == 1
	}

	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(provided), salt, timeCost, memory, threads, uint32(len(decodedHash)))
	return subtle.ConstantTimeCompare(computed, decodedHash) == 1
}
