package main

import (
	"context"
	"github.com/FluxNFTLabs/sdk-go/chain/indexer/account"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcmetadata "google.golang.org/grpc/metadata"
	"strconv"
)

func main() {
	senderPrivKey := ethsecp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	senderAddr, _ := bech32.ConvertAndEncode("lux", senderPrivKey.PubKey().Address().Bytes())

	cc, err := grpc.Dial("localhost:4454", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	accountClient := account.NewAPIClient(cc)

	ctx := context.Background()
	acc, err := accountClient.GetAccount(ctx, &account.GetAccountRequest{Address: senderAddr})
	if err != nil {
		panic(err)
	}

	// prepare header & ctx
	req := account.SetAccountAliasRequest{
		Alias: "hehe2",
	}
	nonce := []byte(strconv.FormatUint(acc.Nonce, 10))
	reqBz, _ := req.Marshal()
	reqHash := ethcrypto.Keccak256(append(reqBz, nonce...))
	senderEthPk, _ := ethcrypto.ToECDSA(senderPrivKey.Bytes())
	reqSig, err := ethcrypto.Sign(reqHash, senderEthPk)
	if err != nil {
		panic(err)
	}

	ctx = grpcmetadata.NewOutgoingContext(ctx, grpcmetadata.MD{
		"sender":    []string{senderAddr},
		"signature": []string{common.Bytes2Hex(reqSig)},
	})

	// set alias
	_, err = accountClient.SetAccountAlias(ctx, &req)
	if err != nil {
		panic(err)
	}
}
