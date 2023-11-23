# cosmos-sdk client v0.50.1
proto signing uses sha256 + cosmos secp256k1
eip712 compat signing uses kecakk256 + eth secp256k1
```
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func main() {
	bz, _ := hex.DecodeString("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")

	// cosmos sig
	pk := secp256k1.PrivKey{Key: bz}
	msg := []byte("hehehe")
	sig1, _ := pk.Sign(msg)
	fmt.Println(hex.EncodeToString(sig1))

	// hand-craft sig
	h := sha256.New()
	h.Write(msg)
	msgHash := h.Sum(nil)

	ethPk, _ := ethcrypto.ToECDSA(bz)
	sig2, _ := ethcrypto.Sign(msgHash, ethPk)
	fmt.Println(hex.EncodeToString(sig2[:64]))
}
```
