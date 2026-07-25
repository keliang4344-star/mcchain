package main

import (
	"fmt"

	bip39 "github.com/cosmos/go-bip39"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

func main() {
	mnemonic := "permit ozone attend skill fog all enable purpose evidence endorse solve album bind box eagle faith tool mad wing furnace slight combine enrich basket"
	seed := bip39.NewSeed(mnemonic, "")
	masterPriv, chainCode := hd.ComputeMastersFromSeed(seed)
	derivedPriv, err := hd.DerivePrivateKeyForPath(masterPriv, chainCode, sdk.GetConfig().GetFullBIP44Path())
	if err != nil {
		panic(err)
	}
	priv := &secp256k1.PrivKey{Key: derivedPriv}
	pub := priv.PubKey()
	cdc := codec.NewLegacyAmino()
	cdc.RegisterInterface((*cryptotypes.PubKey)(nil), nil)
	cdc.RegisterConcrete(&secp256k1.PubKey{}, "tendermint/PubKeySecp256k1", nil)
	aminoBz, _ := cdc.Marshal(pub)
	mcpub, _ := bech32.ConvertAndEncode("mcpub", aminoBz)
	fmt.Println("address:", pub.Address().String())
	fmt.Println("mcpub  :", mcpub)
	fmt.Println("aminoBz:", fmt.Sprintf("%x", aminoBz))
}
