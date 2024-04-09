package golana

import (
	"bytes"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	bin "github.com/gagliardetto/binary"
)

type Rent struct {
	LamportsPerByteYear uint64
	ExemptionPeriod     float64
	BurnPercent         byte
}

var DefaultRent = Rent{
	LamportsPerByteYear: types.DefaultLamportsPerByteYear,
	ExemptionPeriod:     types.DefaultExemptionThreshold,
	BurnPercent:         types.DefaultBurnPercent,
}

func MarshalBinary(x interface{}) ([]byte, error) {
	rentBz := new(bytes.Buffer)
	enc := bin.NewBinEncoder(rentBz)
	if err := enc.Encode(x); err != nil {
		return nil, err
	}
	return rentBz.Bytes(), nil
}
