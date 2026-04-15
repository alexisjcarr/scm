package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b[:]))
}
