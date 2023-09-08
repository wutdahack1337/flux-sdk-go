all:

chain-types:
	rm -rf chain && mkdir chain chain/types chain/app chain/crypto chain/indexer chain/modules chain/modules/fnft chain/modules/fnft/types chain/modules/bazaar chain/modules/bazaar/types chain/stream chain/stream/types
	cp -r ../sdk-go/chain/stream/types/ chain/stream/types
	cp -r ../sdk-go/chain/modules/fnft/types/ chain/modules/fnft/types
	cp -r ../sdk-go/chain/modules/bazaar/types/ chain/modules/bazaar/types
	cp -r ../sdk-go/chain/indexer/ chain/indexer
	cp -r ../sdk-go/chain/crypto/ chain/crypto
	cp -r ../sdk-go/chain/app/ante/ chain/app/ante
	cp -r ../sdk-go/chain/types/ chain/types

	rm chain/crypto/*/*test.go

	echo "👉 Replace sdk-go/chain with sdk-go/chain"
