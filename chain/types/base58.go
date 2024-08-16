package types

import (
	"encoding/json"

	"github.com/cosmos/btcutil/base58"
)

type Base58Bytes []byte

// MarshalJSON encodes the bytes to a Base58 string when serializing to JSON.
func (b Base58Bytes) MarshalJSON() ([]byte, error) {
	base58Str := base58.Encode(b)
	return json.Marshal(base58Str)
}

// UnmarshalJSON decodes a Base58 string to bytes when deserializing from JSON.
func (b *Base58Bytes) UnmarshalJSON(data []byte) error {
	var base58Str string
	if err := json.Unmarshal(data, &base58Str); err != nil {
		return err
	}

	decoded := base58.Decode(base58Str)
	*b = decoded
	return nil
}

func (b Base58Bytes) String() string {
	return base58.Encode(b)
}

func (b Base58Bytes) Bytes() []byte {
	return b.Bytes()
}
