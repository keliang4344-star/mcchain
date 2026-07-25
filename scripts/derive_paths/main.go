package main

import (
	"encoding/base64"
	"fmt"

	bip39 "github.com/cosmos/go-bip39"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// 目标：mcchaind keys add --recover 对 mnemonic 实际恢复出的公钥（base64 of 33 bytes）。
const targetPubB64 = "A5SDVMSWyV0Wf8JAHxbOGXCLfwoK6IuHSaING/MXLgbO"

var mnemonic = "mom merry morning skull behave memory door talent dove enough strike public squirrel play moral vibrant awesome day step scale luxury lab top science"

func derive(path string) []byte {
	seed := bip39.NewSeed(mnemonic, "")
	masterPriv, chainCode := hd.ComputeMastersFromSeed(seed)
	derivedPriv, err := hd.DerivePrivateKeyForPath(masterPriv, chainCode, path)
	if err != nil {
		return nil
	}
	priv := &secp256k1.PrivKey{Key: derivedPriv}
	return priv.PubKey().Bytes()
}

func main() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")

	target, err := base64.StdEncoding.DecodeString(targetPubB64)
	if err != nil {
		panic(err)
	}
	fmt.Printf("target pub bytes: %x\n", target)

	paths := []string{
		"m/44'/118'/0'/0/0",
		"m/44'/118'/0'/0",
		"m/44'/118'/0'/0/1",
		"m/44'/60'/0'/0/0",
		"m/44'/0'/0'/0/0",
		"m/44'/330'/0'/0/0",
		"m/44'/118'/2147483647'/0/0",
		"m/44'/118'/0'/0/0'",
		"m/0'/0'/0'/0/0",
		"m/44'/118'/1'/0/0",
		"m/44'/1'/0'/0/0",
		"m/44'/118'/0'/0/0/0",
		"m/44'/118'/0'/0/0\"",
		"m/44'/994'/0'/0/0",
		"m/44'/529'/0'/0/0",
		"m/44'/750'/0'/0/0",
		"m/44'/886'/0'/0/0",
		"m/44'/118'/0'/0/0",
	}
	for _, p := range paths {
		bz := derive(p)
		if bz == nil {
			fmt.Printf("path %-30s -> DERIVE ERROR\n", p)
			continue
		}
		match := ""
		if string(bz) == string(target) {
			match = "  <<< MATCH"
		}
		fmt.Printf("path %-30s -> %x%s\n", p, bz, match)
	}
}
