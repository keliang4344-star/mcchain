package main

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/tokenomics/types"
)

func main() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")
	fmt.Println(types.TeamAddress.String())
}
