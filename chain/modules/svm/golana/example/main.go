package main

import (
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/golana"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
)

func main() {
	// TODO: compose instructions properly
	for {
		msg := &types.MsgTransaction{
			Sender: "lux1cml96vmptgw99syqrrz8az79xer2pcgp209sv4",
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

		cbCtx := golana.NewMockCallbackContext()
		unitConsumed, logs, err := cbCtx.Execute(msg)
		fmt.Println(unitConsumed, logs, err)
		cbCtx.Done()
	}
}
