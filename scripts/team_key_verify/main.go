package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	bip39 "github.com/cosmos/go-bip39"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"

	// 导入编译进链的常量（与运行链完全一致）
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// vaultMnemonics 来自 team_keys_gen.json（与编译进链的 teamPubKeyStrings 应当一致）。
var vaultMnemonics []string

func loadMnemonics() {
	b, err := os.ReadFile("$HOME/mcchain/team_keys_gen.json")
	if err != nil {
		panic(fmt.Sprintf("read team_keys_gen.json: %v", err))
	}
	var out []struct {
		Mnemonic string `json:"mnemonic"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	if len(out) != 5 {
		panic("team_keys_gen.json must contain exactly 5 entries")
	}
	for _, o := range out {
		vaultMnemonics = append(vaultMnemonics, o.Mnemonic)
	}
}

// aminoMcpub 把 secp256k1 公钥做 amino 编码后转 bech32 mcpub（与 gen_team_keys / 链一致）。
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

// BIP44_PATH 必须与 mcchaind keyring 实际派生路径一致（经验证为 m/44'/118'/0'/0/0）。
const BIP44_PATH = "m/44'/118'/0'/0/0"

// keyringDerive 复刻 mcchaind keyring 的派生（bip39 标准路径 m/44'/118'/0'/0/0）。
func keyringDerive(mnemonic string) cryptotypes.PubKey {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		panic(fmt.Sprintf("invalid mnemonic: %v", err))
	}
	masterPriv, chainCode := hd.ComputeMastersFromSeed(seed)
	derivedPriv, err := hd.DerivePrivateKeyForPath(masterPriv, chainCode, BIP44_PATH)
	if err != nil {
		panic(err)
	}
	priv := &secp256k1.PrivKey{Key: derivedPriv}
	return priv.PubKey()
}

func main() {
	loadMnemonics()
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")
	cfg.SetBech32PrefixForValidator("mcvaloper", "mcvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("mcvalcons", "mcvalconspub")

	ms := tokenomicstypes.TeamMultisigPubKey
	msConcrete, ok := ms.(*multisig.LegacyAminoPubKey)
	if !ok {
		panic("TeamMultisigPubKey is not *multisig.LegacyAminoPubKey")
	}
	msPubs := msConcrete.GetPubKeys()
	compiledTeamAddr := tokenomicstypes.TeamAddress.String()

	fmt.Println("==== 链编译的 TeamAddress (keys.go) ====")
	fmt.Println(compiledTeamAddr)
	fmt.Println("==== 逐个校验 vault 助记词 -> 编译多签 ====")

	derivedPubs := make([]cryptotypes.PubKey, 5)
	allFound := true
	for i, m := range vaultMnemonics {
		pub := keyringDerive(m)
		derivedPubs[i] = pub
		mcpub := aminoMcpub(pub)
		found := false
		for _, mp := range msPubs {
			if mp.Equals(pub) {
				found = true
				break
			}
		}
		if !found {
			allFound = false
		}
		fmt.Printf("team%d  derived_mcpub=%s  in_compiled_multisig=%v\n", i+1, mcpub, found)
	}

	// 用 vault 助记词重建 3-of-5 多签（按地址排序，与链/CLI 一致），比较地址
	sort.Slice(derivedPubs, func(i, j int) bool {
		return string(derivedPubs[i].Address().Bytes()) < string(derivedPubs[j].Address().Bytes())
	})
	rebuilt := multisig.NewLegacyAminoPubKey(int(tokenomicstypes.TeamMultisigThreshold), derivedPubs)
	rebuiltAddr := sdk.AccAddress(rebuilt.Address()).String()

	fmt.Println("==== 结论 ====")
	fmt.Printf("编译 TeamAddress     : %s\n", compiledTeamAddr)
	fmt.Printf("vault 重建 TeamAddress: %s\n", rebuiltAddr)
	if allFound && rebuiltAddr == compiledTeamAddr {
		fmt.Println("✅ 通过：vault 5 个助记词可恢复出编译进链的 3-of-5 多签，团队资金可控。")
	} else {
		fmt.Println("❌ 失败：vault 助记词与链编译多签不一致，团队资金可能永久锁定！")
	}
}
