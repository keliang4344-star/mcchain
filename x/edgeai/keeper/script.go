package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// ScriptSpec 验证脚本规范（白皮书行 490-493）。
//
// 定义链下验证脚本的元数据约束。任务创建时可绑定 ScriptSpec，
// 提交结果时校验脚本哈希是否匹配，确保执行节点使用了指定的验证脚本。
//
// 当前以非 protobuf Go 结构体持久化（JSON 编码），待 proto 重新生成后可迁移。
type ScriptSpec struct {
	ScriptHash     string `json:"script_hash"`
	PythonVersion  string `json:"python_version"`
	TimeoutSeconds uint32 `json:"timeout_seconds"`
	NetworkAllowed bool   `json:"network_allowed"`
}

// script spec key prefix for KV store
var scriptSpecKeyPrefix = []byte("script_spec:")

// task-script binding key prefix: "task_script:" + taskID → scriptHash
var taskScriptKeyPrefix = []byte("task_script:")

func scriptSpecKey(scriptHash string) []byte {
	return append(scriptSpecKeyPrefix, []byte(scriptHash)...)
}

func taskScriptKey(taskID string) []byte {
	return append(taskScriptKeyPrefix, []byte(taskID)...)
}

// SetTaskScriptHash 绑定任务与验证脚本哈希（任务创建时可选调用）。
func (k Keeper) SetTaskScriptHash(ctx sdk.Context, taskID, scriptHash string) {
	ctx.KVStore(k.storeKey).Set(taskScriptKey(taskID), []byte(scriptHash))
}

// GetTaskScriptHash 获取任务绑定的验证脚本哈希；未绑定时返回空字符串。
func (k Keeper) GetTaskScriptHash(ctx sdk.Context, taskID string) string {
	bz := ctx.KVStore(k.storeKey).Get(taskScriptKey(taskID))
	if bz == nil {
		return ""
	}
	return string(bz)
}

// DeleteTaskScriptHash 解除任务与脚本的绑定。
func (k Keeper) DeleteTaskScriptHash(ctx sdk.Context, taskID string) {
	ctx.KVStore(k.storeKey).Delete(taskScriptKey(taskID))
}

// SetScriptSpec 持久化脚本规范（JSON 编码）。
func (k Keeper) SetScriptSpec(ctx sdk.Context, s *ScriptSpec) error {
	bz, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("edgeai: marshal script spec: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(scriptSpecKey(s.ScriptHash), bz)
	return nil
}

// GetScriptSpec 按脚本哈希读取脚本规范；不存在返回 nil。
func (k Keeper) GetScriptSpec(ctx sdk.Context, scriptHash string) (*ScriptSpec, error) {
	bz := ctx.KVStore(k.storeKey).Get(scriptSpecKey(scriptHash))
	if bz == nil {
		return nil, nil
	}
	var s ScriptSpec
	if err := json.Unmarshal(bz, &s); err != nil {
		return nil, fmt.Errorf("edgeai: unmarshal script spec: %w", err)
	}
	return &s, nil
}

// HasScriptSpec 判断指定哈希的脚本规范是否存在。
func (k Keeper) HasScriptSpec(ctx sdk.Context, scriptHash string) bool {
	return ctx.KVStore(k.storeKey).Has(scriptSpecKey(scriptHash))
}

// AllScriptSpecs 返回全部已注册的脚本规范。
func (k Keeper) AllScriptSpecs(ctx sdk.Context) []*ScriptSpec {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), scriptSpecKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*ScriptSpec, 0)
	for ; it.Valid(); it.Next() {
		var s ScriptSpec
		if err := json.Unmarshal(it.Value(), &s); err != nil {
			continue
		}
		out = append(out, &s)
	}
	return out
}

// DeleteScriptSpec 删除指定脚本规范。
func (k Keeper) DeleteScriptSpec(ctx sdk.Context, scriptHash string) {
	ctx.KVStore(k.storeKey).Delete(scriptSpecKey(scriptHash))
}

// ComputeScriptHash 计算脚本内容的 SHA-256 哈希（十六进制编码）。
// 输入 scriptContent 为验证脚本的原始字节（如 .py 文件内容）。
// 返回 hex 编码的 64 字符哈希字符串。
func ComputeScriptHash(scriptContent []byte) string {
	h := sha256.Sum256(scriptContent)
	return hex.EncodeToString(h[:])
}

// RegisterScriptSpec 创建并持久化脚本规范（辅助函数）。
// 自动计算 script_hash，若已存在同哈希则覆盖。
func (k Keeper) RegisterScriptSpec(ctx sdk.Context, scriptContent []byte,
	pythonVersion string, timeoutSeconds uint32, networkAllowed bool,
) (*ScriptSpec, error) {
	scriptHash := ComputeScriptHash(scriptContent)
	spec := &ScriptSpec{
		ScriptHash:     scriptHash,
		PythonVersion:  pythonVersion,
		TimeoutSeconds: timeoutSeconds,
		NetworkAllowed: networkAllowed,
	}
	if err := k.SetScriptSpec(ctx, spec); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.ScriptSpecRegistered",
			sdk.NewAttribute("script_hash", scriptHash),
			sdk.NewAttribute("python_version", pythonVersion),
		),
	)
	return spec, nil
}

// ValidateScriptHash 校验提交的脚本哈希是否与任务绑定的脚本哈希一致。
// 若任务未绑定脚本（scriptHash 为空），直接通过。
// 若任务绑定了脚本但提交的哈希不匹配，返回错误。
func (k Keeper) ValidateScriptHash(ctx sdk.Context, boundHash, submittedHash string) error {
	if boundHash == "" {
		// 任务未绑定验证脚本，跳过校验
		return nil
	}
	if !k.HasScriptSpec(ctx, boundHash) {
		return fmt.Errorf("edgeai: bound script spec %s not registered", boundHash)
	}
	if boundHash != submittedHash {
		return fmt.Errorf("edgeai: script hash mismatch: bound=%s, submitted=%s", boundHash, submittedHash)
	}
	return nil
}

// DefaultScriptSpec 返回使用模块默认参数的脚本规范（用于任务创建时未指定脚本的场景）。
func DefaultScriptSpec(scriptHash string) *ScriptSpec {
	return &ScriptSpec{
		ScriptHash:     scriptHash,
		PythonVersion:  "",
		TimeoutSeconds: types.DefaultScriptTimeout,
		NetworkAllowed: types.DefaultScriptNetworkAllowed,
	}
}
