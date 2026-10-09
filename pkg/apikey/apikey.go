package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"hash/crc32"
	"strings"
)

const (
	DisplayPrefixLen = 12
	BodyLen          = 32
	CheckLen         = 6

	base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

type Issued struct {
	Secret string
	Prefix string
	Hash   [32]byte
}

func Len(prefix string) int { return len(prefix) + BodyLen + CheckLen }

func Hash(secret string) [32]byte { return sha256.Sum256([]byte(secret)) }

func Generate(prefix string) (Issued, error) {
	body := make([]byte, 0, BodyLen)
	buf := make([]byte, 48)
	for len(body) < BodyLen {
		if _, err := rand.Read(buf); err != nil {
			return Issued{}, err
		}
		for _, b := range buf {
			if b < 248 && len(body) < BodyLen {
				body = append(body, base62[b%62])
			}
		}
	}
	check := Checksum(body)
	secret := prefix + string(body) + string(check[:])
	return Issued{Secret: secret, Prefix: secret[:DisplayPrefixLen], Hash: Hash(secret)}, nil
}

func Parse(prefix, presented string) ([32]byte, bool) {
	rest, ok := strings.CutPrefix(presented, prefix)
	if !ok || len(rest) != BodyLen+CheckLen {
		return [32]byte{}, false
	}
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return [32]byte{}, false
		}
	}
	check := Checksum([]byte(rest[:BodyLen]))
	if string(check[:]) != rest[BodyLen:] {
		return [32]byte{}, false
	}
	return Hash(presented), true
}

func Checksum(body []byte) [CheckLen]byte {
	n := crc32.ChecksumIEEE(body)
	var out [CheckLen]byte
	for i := CheckLen - 1; i >= 0; i-- {
		out[i] = base62[n%62]
		n /= 62
	}
	return out
}
