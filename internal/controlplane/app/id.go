package app

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"
)

const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func newID(prefix string) string {
	var raw [16]byte
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(time.Now().UTC().UnixMilli()))
	copy(raw[:6], ts[2:])
	if _, err := rand.Read(raw[6:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return fmt.Sprintf("%s-%s", prefix, encodeULID(raw))
}

func encodeULID(raw [16]byte) string {
	n := new(big.Int).SetBytes(raw[:])
	base := big.NewInt(32)
	mod := new(big.Int)
	encoded := make([]byte, 26)
	for i := len(encoded) - 1; i >= 0; i-- {
		n.DivMod(n, base, mod)
		encoded[i] = crockfordBase32[mod.Int64()]
	}
	return string(encoded)
}
