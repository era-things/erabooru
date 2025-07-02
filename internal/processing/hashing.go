package processing

import (
	"encoding/hex"
	"io"
	"os"

	"github.com/zeebo/xxh3"
)

// HashFile128Hex opens the file at path, computes its XXH3 128-bit hash,
// and returns the result as a lowercase hexadecimal string.
func HashFile128Hex(file *os.File) (string, error) {
	h := xxh3.New() // streaming 128-bit hasher
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	u128 := h.Sum128() // returns a Uint128 (Hi, Lo) :contentReference[oaicite:0]{index=0}
	b := u128.Bytes()  // [16]byte big-endian canonical form :contentReference[oaicite:1]{index=1}

	return hex.EncodeToString(b[:]), nil // convert to hex string
}
