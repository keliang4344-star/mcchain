package app_test

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"

	"mcchain/app"
)

// 创世校验严格性：`validate-genesis` 与 `start` 必须对
// genesis.json 用**同一个**编解码器，否则验收就是假 PASS。
//
// 事故形态：x/tokenomics 的 ValidateGenesis 用 encoding/json（宽松，未知字段静默
// 忽略），而 InitGenesis 用 cdc.MustUnmarshalJSON（ProtoCodec / jsonpb，未知字段
// 直接 panic）。于是一份多写了 "params": {} 的 genesis.json 会得到：
//
//	$ mcchaind validate-genesis   →  File ... is a valid genesis file   ← 绿灯
//	$ mcchaind start              →  panic: unknown field "params" ...  ← 起不来
//
// 上线当天遇到这个，现场只剩一句没有模块名、没有行号的 panic。
//
// 本测试逐模块验证：给默认创世塞一个 schema 里不存在的字段，ValidateGenesis
// 必须报错。能报错 ⇔ 它与 InitGenesis 同口径 ⇔ 验收结果可信。
func TestValidateGenesisRejectsUnknownFields(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	// 只覆盖本仓库自有的 proto 模块（SDK 内置模块的编解码行为不在我们控制范围内；
	// liquidstaking 的 GenesisState 是手写结构体、InitGenesis 同样走 encoding/json，
	// 两侧本就同口径，不在此列）。
	names := []string{
		"tokenomics", "depin", "dex", "edgeai", "phonenode", "referral", "mcchain",
	}

	for _, name := range names {
		amb, ok := app.ModuleBasics[name]
		require.True(t, ok, "模块 %s 未注册在 ModuleBasics", name)
		mb, ok := amb.(module.HasGenesisBasics)
		require.True(t, ok, "模块 %s 未实现 HasGenesisBasics（无法在创世阶段校验）", name)

		def := mb.DefaultGenesis(cdc)
		require.NotEmpty(t, def, "模块 %s 的默认创世为空", name)

		var obj map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(def, &obj), "模块 %s 默认创世不是合法 JSON 对象", name)

		obj["__strictgen_unknown_field__"] = json.RawMessage(`1`)
		bad, err := json.Marshal(obj)
		require.NoError(t, err)

		verr := mb.ValidateGenesis(cdc, nil, bad)
		require.Error(t, verr,
			"模块 %s 的 ValidateGenesis 接受了 schema 之外的字段 —— 它与 InitGenesis 不同口径，"+
				"会导致 validate-genesis 报 valid 而 start 当场 panic（上线当天才发现）", name)
	}
}
