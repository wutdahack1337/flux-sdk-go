package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/app/ante/typeddata"
	"github.com/FluxNFTLabs/sdk-go/chain/crypto/ethsecp256k1"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/web3gw"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
)

type Fee struct {
	Amount   sdk.Coins `json:"amount"`
	FeePayer string    `json:"feePayer"`
	Gas      string    `json:"gas"`
}

func main() {
	senderAddress, cosmosKeyring, err := chainclient.InitCosmosKeyring(
		os.Getenv("HOME")+"/.fluxd",
		"fluxd",
		"file",
		"user1",
		"12345678",
		"88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305",
		false,
	)
	if err != nil {
		panic(err)
	}
	clientCtx, err := chaintypes.NewClientContext(
		"flux-1",
		senderAddress.String(),
		cosmosKeyring,
	)
	if err != nil {
		panic(err)
	}

	// init grpc client
	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewWeb3GWClient(cc)

	// init msg
	msg := &banktypes.MsgSend{
		FromAddress: senderAddress.String(),
		ToAddress:   "lux1jcltmuhplrdcwp7stlr4hlhlhgd4htqhu86cqx",
		Amount: sdk.Coins{
			{Denom: "lux", Amount: sdk.NewIntFromUint64(77)},
		},
	}
	protoCodec := codec.NewProtoCodec(clientCtx.InterfaceRegistry)
	msgBytes, err := protoCodec.MarshalInterfaceJSON(msg)
	if err != nil {
		panic(err)
	}

	// prepare tx
	prepareRes, err := client.PrepareTx(context.Background(), &types.PrepareTxRequest{
		ChainId:       1,
		SignerAddress: common.Bytes2Hex(senderAddress.Bytes()),
		Memo:          "",
		TimeoutHeight: 0,
		Fee: &types.Fee{
			Price:    sdk.Coins{{Denom: "lux", Amount: sdk.NewIntFromUint64(500000)}},
			Delegate: true,
		},
		Msgs: [][]byte{msgBytes},
	})
	if err != nil {
		panic(err)
	}

	// sign tx using eip712
	var typedData typeddata.TypedData
	err = json.Unmarshal([]byte(prepareRes.Data), &typedData)
	if err != nil {
		panic(err)
	}
	hash, err := typeddata.ComputeTypedDataHash(typedData)
	if err != nil {
		panic(err)
	}
	privKey := ethsecp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	sig, err := privKey.Sign(hash)
	if err != nil {
		panic(err)
	}
	sigV2 := signing.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: sig,
		},
		Sequence: 1,
	}

	bz, err := json.Marshal(typedData.Message["fee"])
	if err != nil {
		panic(err)
	}
	var fee Fee
	err = json.Unmarshal(bz, &fee)
	if err != nil {
		panic(err)
	}

	// build std tx
	stdTxConfig := legacytx.StdTxConfig{Cdc: codec.NewLegacyAmino()}
	stdTxBuilder := stdTxConfig.NewTxBuilder()
	stdTxBuilder.SetFeeGranter(senderAddress)
	stdTxBuilder.SetFeePayer(sdk.MustAccAddressFromBech32(prepareRes.FeePayer))
	stdTxBuilder.SetMsgs(msg)
	stdTxBuilder.SetSignatures(sigV2)
	stdTxBuilder.SetFeeAmount(fee.Amount)
	stdTxBuilder.SetGasLimit(200000)

	return legacytx.StdSignBytes(
		data.ChainID, data.AccountNumber, data.Sequence, protoTx.GetTimeoutHeight(),
		legacytx.StdFee{
			Amount:  protoTx.GetFee(),
			Gas:     protoTx.GetGas(),
			Payer:   protoTx.tx.AuthInfo.Fee.Payer,
			Granter: protoTx.tx.AuthInfo.Fee.Granter,
		},
		tx.GetMsgs(), protoTx.GetMemo(), tip,

	stdTx := stdTxBuilder.GetTx().S
	txBytes, err := json.Marshal(stdTx)
	if err != nil {
		panic(err)
	}

	// broadcast tx

	fmt.Println(string(txBytes))

	broadcastRes, err := client.BroadcastTx(context.Background(), &types.BroadcastTxRequest{
		ChainId: 1,
		Tx:      txBytes,
		Msgs:    [][]byte{msgBytes},
		PubKey: &types.PubKey{
			Type: privKey.PubKey().Type(),
			Key:  privKey.PubKey().String(),
		},
		Sig:         string(sig),
		FeePayer:    prepareRes.FeePayer,
		FeePayerSig: prepareRes.FeePayerSig,
		Mode:        prepareRes.SignMode,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(broadcastRes)

}
