package main

import (
	"context"
	"cosmossdk.io/api/cosmos/crypto/secp256k1"
	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"fmt"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/cosmos/cosmos-proto/anyutil"
	sdksecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtxconfig "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// prepare info
	senderPrivKey := sdksecp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	senderPubKey := senderPrivKey.PubKey()
	pk := &secp256k1.PubKey{Key: senderPrivKey.PubKey().Bytes()} // WARN: must use cosmossdk.io/api/cosmos/crypto/secp256k1
	senderPubKeyAny, _ := anyutil.New(pk)
	senderAddr := sdk.AccAddress(senderPrivKey.PubKey().Address().Bytes())
	receiverAddr := sdk.MustAccAddressFromBech32("lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx")

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, _, err := chaintypes.NewClientContext("flux-1", "", nil)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
	)

	// init msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddr.String(),
		ToAddress:   receiverAddr.String(),
		Amount: sdk.Coins{
			{Denom: "lux", Amount: sdkmath.NewIntFromUint64(77)},
		},
	}

	// init tx builder
	senderNum, senderSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, senderAddr)
	if err != nil {
		panic(err)
	}

	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_TEXTUAL},
		TextualCoinMetadataQueryFn: authtxconfig.NewGRPCCoinMetadataQueryFn(cc),
	}
	txConfig, err := authtx.NewTxConfigWithOptions(
		clientCtx.Codec,
		txConfigOpts,
	)
	if err != nil {
		panic(err)
	}
	extTxBuilder, ok := txConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		panic("cannot cast txBuilder")
	}

	// prepare tx data
	timeoutHeight := uint64(19000)
	gasLimit := uint64(200000)
	gasPrice := sdkmath.NewIntFromUint64(500000000)
	fee := []sdk.Coin{{
		Denom:  "lux",
		Amount: sdkmath.NewIntFromUint64(gasLimit).Mul(gasPrice),
	}}
	senderSigV2 := signingtypes.SignatureV2{
		PubKey: senderPubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_TEXTUAL,
			Signature: nil,
		},
		Sequence: senderSeq,
	}

	// build tx
	extTxBuilder.SetMsgs(msg)
	extTxBuilder.SetGasLimit(gasLimit)
	extTxBuilder.SetTimeoutHeight(timeoutHeight)
	extTxBuilder.SetMemo("abc")
	extTxBuilder.SetFeeAmount(fee)
	extTxBuilder.SetSignatures(senderSigV2)

	// build sign data
	signerData := signing.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: senderNum,
		Sequence:      senderSeq,
		PubKey:        senderPubKeyAny,
		Address:       senderAddr.String(),
	}
	data, err := txConfig.SignModeHandler().GetSignBytes(
		context.Background(),
		signingv1beta1.SignMode_SIGN_MODE_TEXTUAL,
		signerData,
		extTxBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
	)
	if err != nil {
		panic(err)
	}

	// TODO: pkg doesn't support to decode screens, we might want to use this on metamask later on
	fmt.Println(string(data))

	senderSig, err := senderPrivKey.Sign(data)
	if err != nil {
		panic(err)
	}

	// set signatures again
	senderSigV2.Data.(*signingtypes.SingleSignatureData).Signature = senderSig
	extTxBuilder.SetSignatures(senderSigV2)

	// broadcast tx
	txBytes, err := clientCtx.TxConfig.TxEncoder()(extTxBuilder.GetTx())
	if err != nil {
		panic(err)
	}
	txRes, err := chainClient.SyncBroadcastSignedTx(txBytes)
	if err != nil {
		panic(err)
	}

	fmt.Println(txRes)
}
