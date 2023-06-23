all:

chain-types:
	rm -rf chain && mkdir chain chain/types chain/fnft chain/fnft/types chain/crypto chain/crypto/ethsecp256k1
	cp ../sdk-go/chain/types/*.go chain/types
	cp ../fluxd/chain/crypto/ethsecp256k1/*.go chain/crypto/ethsecp256k1
	cp ../fluxd/chain/modules/fnft/types/*.go chain/fnft/types
	rm -rf chain/fnft/types/*test.go  rm -rf chain/modules/fnft/types/*gw.go

	echo "👉 Replace fluxd/chain/modules with sdk-go/chain"
	echo "👉 Replace sdk-go/chain/types with sdk-go/chain/types"
