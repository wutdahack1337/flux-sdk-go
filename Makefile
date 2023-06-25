all:

chain-types:
	rm -rf chain && mkdir chain chain/types chain/fnft chain/fnft/types chain/crypto chain/crypto/ethsecp256k1 chain/crypto/codec chain/crypto/hd

	cp ../fluxd/chain/types/*.go chain/types

	cp ../fluxd/chain/crypto/ethsecp256k1/*.go chain/crypto/ethsecp256k1
	cp ../fluxd/chain/crypto/codec/*.go chain/crypto/codec

	cp ../fluxd/chain/crypto/hd/*.go chain/crypto/hd
	rm -rf chain/crypto/hd/*test.go

	cp ../fluxd/chain/modules/fnft/types/*.go chain/fnft/types
	rm -rf chain/fnft/types/*test.go  rm -rf chain/modules/fnft/types/*gw.go

	echo "👉 Replace fluxd/chain with sdk-go/chain"
