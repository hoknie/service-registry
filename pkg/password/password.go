package password

import (
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/alexedwards/argon2id"
)

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint32
}

type Hasher struct {
	params        argon2id.Params
	dummyOnce     sync.Once
	dummy         string
	verifications atomic.Uint64
}

func NewHasher(p Params) *Hasher {
	return &Hasher{params: argon2id.Params{
		Memory: p.MemoryKiB, Iterations: p.Iterations, Parallelism: uint8(min(p.Parallelism, 255)),
		SaltLength: 16, KeyLength: 32,
	}}
}

func (h *Hasher) Hash(password string) (string, error) {
	if !sane(&h.params) {
		return "", fmt.Errorf("password hashing failed: invalid argon2 parameters %+v", h.params)
	}
	out, err := argon2id.CreateHash(password, &h.params)
	if err != nil {
		return "", fmt.Errorf("password hashing failed: %w", err)
	}
	return out, nil
}

func (h *Hasher) Verify(password, phc string) bool {
	h.verifications.Add(1)
	params, _, _, err := argon2id.DecodeHash(phc)
	if err != nil || !sane(params) {
		return false
	}
	ok, err := argon2id.ComparePasswordAndHash(password, phc)
	return err == nil && ok
}

func (h *Hasher) VerifyDummy(password string) {
	h.dummyOnce.Do(func() { h.dummy, _ = h.Hash(rand.Text()) })
	_ = h.Verify(password, h.dummy)
}

func (h *Hasher) Verifications() uint64 { return h.verifications.Load() }

func sane(p *argon2id.Params) bool {
	return p.Iterations >= 1 && p.Parallelism >= 1 && uint64(p.Memory) >= 8*uint64(p.Parallelism) &&
		p.SaltLength >= 8 && p.KeyLength >= 4
}
