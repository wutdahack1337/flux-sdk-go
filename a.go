package main

import (
	"fmt"
	"github.com/goccy/go-json"
)

type Fee struct {
	FeePayer string `json:"feePayer"`
	Gas      string `json:"gas"`
}

func main() {
	bz := []byte(`{"feePayer":"lux1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hdef8k5","gas":"77707"}`)

	var fee Fee
	err := json.Unmarshal(bz, &fee)
	if err != nil {
		panic(err)
	}

	fmt.Println(fee)
}
