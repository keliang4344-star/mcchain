package tokenomics

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/tokenomics/types"
)

// GenerateGenesisState 生成随机创世状态（仿真用）。
// 注意：tokenomics 模块的 InitGenesis/ExportGenesis/DefaultGenesis 统一使用
// encoding/json（uint64 序列化为数字），故此处也必须用 encoding/json 生成，
// 与 InitGenesis 的 json.Unmarshal 保持一致，否则仿真初始化会因
// "string -> uint64" 解码失败而 panic。
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	tokenomicsGenesis := types.DefaultGenesis()
	bz, err := json.Marshal(tokenomicsGenesis)
	if err != nil {
		panic(err)
	}
	simState.GenState[types.ModuleName] = bz
}

// RegisterStoreDecoder 注册 store 解码器（本模块无专用解码器）。
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// ProposalContents 返回治理提案内容（本模块无）。
func (AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalContent {
	return nil
}

// WeightedOperations 返回仿真操作（本模块无 Msg，无操作）。
func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

// ProposalMsgs 返回仿真治理提案消息（本模块无 Msg）。
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
