package main

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

func aminoMcpub(pub cryptotypes.PubKey) string {
	cdc := codec.NewLegacyAmino()
	cdc.RegisterInterface((*cryptotypes.PubKey)(nil), nil)
	cdc.RegisterConcrete(&secp256k1.PubKey{}, "tendermint/PubKeySecp256k1", nil)
	aminoBz, err := cdc.Marshal(pub)
	if err != nil {
		panic(err)
	}
	mcpub, err := bech32.ConvertAndEncode("mcpub", aminoBz)
	if err != nil {
		panic(err)
	}
	return mcpub
}

func main() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")

	// mcchaind 恢复 "mom merry morning..." 得到的原始公钥（seed_cmp 验证）
	raw, _ := hex.DecodeString("03948354c496c95d167fc2401f16ce19708b7f0a0ae88b8749a20d1bf3172e06ce")
	pub := &secp256k1.PubKey{Key: raw}
	got := aminoMcpub(pub)
	compiled := "mcpub1addwnpepqw2gx4xyjmy469nlcfqp79kwr9cgklc2pt5ghp6f5gx3huch9crvuwuxm8p"
	fmt.Printf("aminoMcpub(03948354) = %s\n", got)
	fmt.Printf("compiled team1 mcpub  = %s\n", compiled)
	if got == compiled {
		fmt.Println("✅ 一致：编译的 team1 公钥 == mcchaind 恢复出的公钥（03948354...）")
	} else {
		fmt.Println("❌ 不一致")
	}
	fmt.Printf("pubkey address (bech32) = %s\n", sdk.AccAddress(pub.Address()).String())
}
