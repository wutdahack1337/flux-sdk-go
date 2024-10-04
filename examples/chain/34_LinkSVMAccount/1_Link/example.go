package main

import (
	"context"
	"fmt"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	network := common.LoadNetwork("local", "")
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

	// user1, user2, user3
	pks := []string{"88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305", "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544", "39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608"}
	svmPks := []string{"4jiFsPBB32qc55FUisywGkxfrFaoakYCyUo9chd8ys5EVjTbGkFi7ztnHF7dnCx77tjQsd1Kg1m63XWhPhXeSYLn", "afQDa9e9tZiKQPdxu6BBBkwgLRRLVZFykjvJZWLgKUpw685XzdhCK1qjRRT4FscXxDyKwjknfZVPQG75PaT7vzd", "3uEdHTmgWcU2iMp7msjZ777LCB1qvwW1WcFoJof5hzFzusg6wJ7XZZjA77scbwrrscSYEf1zuiqn2kod3q853b7A"}
	for i, pk := range pks {
		// prepare users
		userPrivKey := ethsecp256k1.PrivKey{Key: ethcommon.Hex2Bytes(pk)}
		userPubKey := userPrivKey.PubKey()
		userAddr := sdk.AccAddress(userPubKey.Address().Bytes())
		svmKeypair, err := solana.PrivateKeyFromBase58(svmPks[i])
		if err != nil {
			panic(err)
		}

		// init link msg
		svmPubkey := svmKeypair.PublicKey()
		svmSig, err := svmKeypair.Sign([]byte(userAddr.String()))
		if err != nil {
			panic(err)
		}
		msg := &svmtypes.MsgLinkSVMAccount{
			Sender:       userAddr.String(),
			SvmPubkey:    svmPubkey.Bytes(),
			SvmSignature: svmSig[:],
			Amount:       sdk.NewInt64Coin("lux", 1000000000),
		}
		// link and fund user1, link user2 user3
		if i == 0 {
			msg.Amount = sdk.Coin{
				Denom:  "lux",
				Amount: sdkmath.NewIntFromUint64(1000000000000),
			}
		}
		fmt.Println(fmt.Sprintf("linking %s to %s", userAddr.String(), svmKeypair.PublicKey().String()))

		// init tx builder
		userNum, userSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, userAddr)
		if err != nil {
			panic(err)
		}
		txConfig := chaintypes.NewTxConfig([]signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
		txBuilder := txConfig.NewTxBuilder()

		// prepare tx data
		timeoutHeight := uint64(19000)
		gasLimit := uint64(3000000)
		gasPrice := sdkmath.NewIntFromUint64(500000000)
		fee := []sdk.Coin{{
			Denom:  "lux",
			Amount: sdkmath.NewIntFromUint64(gasLimit).Mul(gasPrice),
		}}
		userSigV2 := signingtypes.SignatureV2{
			PubKey: userPubKey,
			Data: &signingtypes.SingleSignatureData{
				SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
				Signature: nil,
			},
			Sequence: userSeq,
		}

		// build tx
		txBuilder.SetMsgs(msg)
		txBuilder.SetGasLimit(gasLimit)
		txBuilder.SetTimeoutHeight(timeoutHeight)
		txBuilder.SetMemo("abc")
		txBuilder.SetFeeAmount(fee)
		txBuilder.SetSignatures(userSigV2)

		// build sign data
		signerData := signing.SignerData{
			ChainID:       clientCtx.ChainID,
			AccountNumber: userNum,
			Sequence:      userSeq,
			Address:       userAddr.String(),
		}
		data, err := txConfig.SignModeHandler().GetSignBytes(
			context.Background(),
			signingv1beta1.SignMode_SIGN_MODE_DIRECT,
			signerData,
			txBuilder.(authsigning.V2AdaptableTx).GetSigningTxData(),
		)
		if err != nil {
			panic(err)
		}
		userSig, err := userPrivKey.Sign(data)
		if err != nil {
			panic(err)
		}

		// set signatures again
		userSigV2.Data.(*signingtypes.SingleSignatureData).Signature = userSig
		txBuilder.SetSignatures(userSigV2)

		// broadcast tx
		txBytes, err := clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
		if err != nil {
			panic(err)
		}
		txResp, err := chainClient.SyncBroadcastSignedTx(txBytes)
		if err != nil {
			panic(err)
		}

		fmt.Println("txHash:", txResp.TxResponse.TxHash)
		fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)
	}
}
