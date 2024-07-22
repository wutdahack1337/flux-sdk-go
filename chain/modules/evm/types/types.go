package types

const (
	// ModuleName defines the name of the nft module
	ModuleName = "evm"

	// StoreKey is the default store key for nft
	StoreKey = ModuleName

	// RouterKey is the message route for nft
	RouterKey = ModuleName

	HashLen       = 32
	EthAddressLen = 20
)

type Result struct {
	Output     []byte
	GasLeft    int64
	GasRefund  int64
	StatusCode string
}
