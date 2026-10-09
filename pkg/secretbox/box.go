package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const version = "v1"

var (
	ErrNoKey      = errors.New("no encryption key configured")
	ErrUnknownKey = errors.New("ciphertext key is not configured")
	ErrCorrupt    = errors.New("ciphertext is invalid")
)

type Key struct {
	ID  string
	Key [32]byte
}

type Box struct {
	keys []Key
}

func (k Key) String() string { return "SecretKey{" + k.ID + "}" }

func (k Key) GoString() string { return k.String() }

func New(keys []Key) *Box { return &Box{keys: keys} }

func (b *Box) CanEncrypt() bool { return len(b.keys) > 0 }

func (b *Box) ActiveKeyID() string {
	if len(b.keys) == 0 {
		return ""
	}
	return b.keys[0].ID
}

func AAD(table, id, column string) []byte { return []byte(table + ":" + id + ":" + column) }

func gcm(key [32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (b *Box) Seal(plain string, aad []byte) (string, error) {
	if len(b.keys) == 0 {
		return "", ErrNoKey
	}
	k := b.keys[0]
	aead, err := gcm(k.Key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), aad)
	return version + "." + k.ID + "." + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func KeyID(ciphertext string) (string, bool) {
	parts := strings.SplitN(ciphertext, ".", 3)
	if len(parts) != 3 || parts[0] != version {
		return "", false
	}
	return parts[1], true
}

func (b *Box) Open(ciphertext string, aad []byte) (string, error) {
	parts := strings.SplitN(ciphertext, ".", 3)
	if len(parts) != 3 || parts[0] != version {
		return "", ErrCorrupt
	}
	var key *Key
	for i := range b.keys {
		if b.keys[i].ID == parts[1] {
			key = &b.keys[i]
		}
	}
	if key == nil {
		return "", ErrUnknownKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", ErrCorrupt
	}
	aead, err := gcm(key.Key)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", ErrCorrupt
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], aad)
	if err != nil {
		return "", ErrCorrupt
	}
	return string(plain), nil
}

func Fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:2])
}
