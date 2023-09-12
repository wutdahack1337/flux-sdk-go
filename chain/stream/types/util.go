package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/gogoproto/proto"
)

func ToAny(msg proto.Message) *codectypes.Any {
	anyMsg, err := codectypes.NewAnyWithValue(msg)
	anyMsg.TypeUrl = anyMsg.TypeUrl[1:]
	if err != nil {
		panic(err)
	}
	return anyMsg
}
