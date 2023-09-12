all:

chain-types:
	rm -rf chain && mkdir chain chain/types chain/app chain/crypto chain/indexer chain/modules chain/modules/fnft chain/modules/fnft/types chain/modules/bazaar chain/modules/bazaar/types chain/stream chain/stream/types
	cp -r ../fluxd/chain/stream/types/ chain/stream/types
	cp -r ../fluxd/chain/modules/fnft/types/ chain/modules/fnft/types
	cp -r ../fluxd/chain/modules/bazaar/types/ chain/modules/bazaar/types
	cp -r ../fluxd/chain/indexer/ chain/indexer
	cp -r ../fluxd/chain/crypto/ chain/crypto
	cp -r ../fluxd/chain/app/ante/ chain/app/ante
	cp -r ../fluxd/chain/types/ chain/types
	rm chain/crypto/*/*test.go
	./scripts/replace_path.sh
