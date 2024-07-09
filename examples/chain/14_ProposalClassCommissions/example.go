package main

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	bazaartypes "github.com/FluxNFTLabs/sdk-go/chain/modules/bazaar/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"strings"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678"),
		chainclient.GetCryptoCodec(),
	)
	if err != nil {
		panic(err)
	}

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"genesis",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		fmt.Println(err)
	}

	// prepare tx msg
	content := &bazaartypes.ClassCommissionsProposal{
		Title:       "Update class commission proposal",
		Description: "09.15.2023",
		ClassCommissions: []bazaartypes.ClassCommission{
			{ClassId: "series", CommissionMul: 77, CommissionDiv: 100},
			{ClassId: "livestream", CommissionMul: 45, CommissionDiv: 100},
		},
	}
	contentAny, err := codectypes.NewAnyWithValue(content)
	if err != nil {
		panic(err)
	}

	depositAmount, _ := sdkmath.NewIntFromString("500000000000000000000")
	proposalMsg := &govtypes.MsgSubmitProposal{
		Content:        contentAny,
		InitialDeposit: sdk.Coins{{Denom: "lux", Amount: depositAmount}},
		Proposer:       senderAddress.String(),
	}

	// prepare vote msg using node account
	voteMsg := &govtypes.MsgVote{
		ProposalId: 1,
		Voter:      senderAddress.String(),
		Option:     govtypes.OptionYes,
	}

	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	err = chainClient.QueueBroadcastMsg(proposalMsg)
	if err != nil {
		fmt.Println(err)
	}
	chainClient.BroadcastDone()

	err = chainClient.QueueBroadcastMsg(voteMsg)
	if err != nil {
		fmt.Println(err)
	}
	chainClient.BroadcastDone()
}
