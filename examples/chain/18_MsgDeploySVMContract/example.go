package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678\n"),
		chainclient.GetCryptoCodec(),
	)
	if err != nil {
		panic(err)
	}

	// init grpc connection
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user1",
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
		panic(err)
	}

	msg1 := &types.MsgTransaction{
		Sender: senderAddress.String(),
		Accounts: []string{
			"5u3ScQH8YNWoWgjuyV2218d4V1HtQSoKf65JpuXXwXVK",
			"CLfvh1736T8KBUWBqSNypizgL5KdZUekJ26gFXV3Lra1",
			"8BTVbEdAFbqsEsjngmaMByn1m9j8jDFtEFFusEwGeMZY",
			"11111111111111111111111111111111",
			"BPFLoaderUpgradeab1e11111111111111111111111",
		},
		Instructions: []*types.Instruction{
			{
				ProgramIndex: []uint32{3},
				Accounts: []*types.InstructionAccount{
					{
						IdIndex:     0,
						CallerIndex: 0,
						CalleeIndex: 0,
						IsSigner:    true,
						IsWritable:  true,
					},
					{
						IdIndex:     1,
						CallerIndex: 1,
						CalleeIndex: 1,
						IsSigner:    true,
						IsWritable:  true,
					},
				},
				Data: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 184, 13, 1, 0, 0, 0, 0, 0, 2, 168, 246, 145, 78, 136, 161, 176, 226, 16, 21, 62, 247, 99, 174, 43, 0, 194, 185, 61, 22, 193, 36, 210, 192, 83, 122, 16, 4, 128, 0, 0},
			},
			{
				ProgramIndex: []uint32{3},
				Accounts: []*types.InstructionAccount{
					{
						IdIndex:     0,
						CallerIndex: 0,
						CalleeIndex: 0,
						IsSigner:    true,
						IsWritable:  true,
					},
					{
						IdIndex:     2,
						CallerIndex: 2,
						CalleeIndex: 1,
						IsSigner:    true,
						IsWritable:  true,
					},
				},
				Data: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 36, 0, 0, 0, 0, 0, 0, 0, 2, 168, 246, 145, 78, 136, 161, 176, 226, 16, 21, 62, 247, 99, 174, 43, 0, 194, 185, 61, 22, 193, 36, 210, 192, 83, 122, 16, 4, 128, 0, 0},
			},
			{
				ProgramIndex: []uint32{4},
				Accounts: []*types.InstructionAccount{
					{
						IdIndex:     1,
						CallerIndex: 1,
						CalleeIndex: 0,
						IsSigner:    true,
						IsWritable:  true,
					},
					{
						IdIndex:     0,
						CallerIndex: 0,
						CalleeIndex: 1,
						IsSigner:    true,
						IsWritable:  true,
					},
				},
				Data: []byte{0, 0, 0, 0},
			},
		},
		ComputeBudget: 1000000,
	}

	res, err := chainClient.SyncBroadcastSvmMsg(msg1)

	fmt.Println(res, err)
}
