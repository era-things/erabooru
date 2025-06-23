package db

import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
)

type HashID [16]byte

func (h *HashID) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok || len(b) != 16 {
		return errors.New("invalid HashID")
	}
	copy(h[:], b)
	return nil
}

func (h HashID) Value() (driver.Value, error) {
	return h[:], nil
}

func (h HashID) String() string {
	return hex.EncodeToString(h[:])
}
