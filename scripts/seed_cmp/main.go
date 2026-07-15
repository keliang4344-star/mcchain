package main

import (
	"encoding/json"
	"fmt"
	"os"

	bip39 "github.com/cosmos/go-bip39"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const mnemonic = "mom merry morning skull behave memory door talent dove enough strike public squirrel play moral vibrant awesome day step scale luxury lab top science"

func deriveFromSeed(seed []byte) string {
	masterPriv, chainCode := hd.ComputeMastersFromSeed(seed)
	derivedPriv, _ := hd.DerivePrivateKeyForPath(masterPriv, chainCode, "m/44'/118'/0'/0/0")
	priv := &secp256k1.PrivKey{Key: derivedPriv}
	return fmt.Sprintf("%x", priv.PubKey().Bytes())
}

func main() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")

	seedNew := bip39.NewSeed(mnemonic, "")
	seedErr, errE := bip39.NewSeedWithErrorChecking(mnemonic, "")
	fmt.Printf("hardcoded + NewSeed             -> %x  (err=%v)\n", seedNew, nil)
	fmt.Printf("hardcoded + NewSeedWErrChecking -> %x  (err=%v)\n", seedErr, errE)
	fmt.Printf("  derived(NewSeed)             -> %s\n", deriveFromSeed(seedNew))
	fmt.Printf("  derived(NewSeedWErr)          -> %s\n", deriveFromSeed(seedErr))

	// JSON-read mnemonic
	b, _ := os.ReadFile("$HOME/mcchain/team_keys_gen.json")
	var arr []struct {
		Mnemonic string `json:"mnemonic"`
	}
	json.Unmarshal(b, &arr)
	jm := arr[0].Mnemonic
	fmt.Printf("json mnemonic == hardcoded? %v\n", jm == mnemonic)
	fmt.Printf("json mnemonic bytes: %q\n", jm)
	seedJ, _ := bip39.NewSeedWithErrorChecking(jm, "")
	fmt.Printf("json + NewSeedWErrChecking     -> %x\n", seedJ)
	fmt.Printf("  derived(json seed)           -> %s\n", deriveFromSeed(seedJ))
	fmt.Printf("TARGET (mcchaind recovered)    -> 03948354c496c95d167fc2401f16ce19708b7f0a0ae88b8749a20d1bf3172e06ce\n")
}
