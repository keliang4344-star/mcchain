package keeper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"mcchain/x/phonenode/types"
)

// ---------------------------------------------------------------------------
// Path C：手机-云端共签（增强层，可选项，2026-08 落地）
//
// 移动节点（手机）本地签名后，云端共签方（CloudSigner）再对相同载荷哈希签名，
// 链上校验云端签名，实现「手机 + 云端」双因素共签。该能力为增强层（非主网必需），
// 旨在提升关键操作（如出块授权、大额调度）的抗篡改与抗单点失控能力。
//
// 数据流：
//   1. RegisterCloudSigner：节点绑定一个云端共签方公钥（secp256k1 原始 33 字节 hex）。
//   2. SubmitCosign：节点提交云端对载荷哈希的签名；链上用绑定公钥验签，
//      通过则持久化共签证明并发出事件，供上层（出块/调度）查询「已双签」。
// ---------------------------------------------------------------------------

var (
	// CloudSignerKeyPrefix 节点 → 云端共签方绑定前缀：CloudSigner:<node>
	CloudSignerKeyPrefix = []byte("CloudSigner:")
	// CosignAttestKeyPrefix 节点最近一次共签证明前缀：CosignAtt:<node>
	CosignAttestKeyPrefix = []byte("CosignAtt:")
)

// CloudSigner 记录某节点绑定的云端共签方（公钥以 hex 存储）。
type CloudSigner struct {
	Node        string `json:"node"`
	CloudPubKey string `json:"cloud_pub_key"` // secp256k1 原始 33 字节 hex
	Registered  bool   `json:"registered"`
}

// CosignAttestation 链上持久化的云端共签证明。
type CosignAttestation struct {
	Node        string `json:"node"`
	PayloadHash string `json:"payload_hash"` // 32 字节 hex
	CloudSig    string `json:"cloud_sig"`    // 64 字节 hex secp256k1 签名（对 payloadHash 原始字节）
	BlockHeight int64  `json:"block_height"`
}

func cloudSignerKey(node string) []byte { return append(CloudSignerKeyPrefix, []byte(node)...) }
func cosignKey(node string) []byte      { return append(CosignAttestKeyPrefix, []byte(node)...) }

// RegisterCloudSigner 为节点绑定云端共签方公钥（Path C 启用）。
// cloudPubKey 为 secp256k1 原始 33 字节 hex。
func (k Keeper) RegisterCloudSigner(ctx sdk.Context, node, cloudPubKey string) error {
	if _, err := sdk.AccAddressFromBech32(node); err != nil {
		return types.ErrInvalidNode.Wrap(node)
	}
	pubBytes, err := hex.DecodeString(cloudPubKey)
	if err != nil || len(pubBytes) != 33 {
		return types.ErrInvalidPubKey.Wrap("cloud_pub_key must be 33-byte hex")
	}
	cs := CloudSigner{Node: node, CloudPubKey: cloudPubKey, Registered: true}
	bz, err := json.Marshal(cs)
	if err != nil {
		return fmt.Errorf("phonenode: marshal cloud signer: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(cloudSignerKey(node), bz)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.CloudSignerRegistered",
		sdk.NewAttribute("node", node),
	))
	return nil
}

// GetCloudSigner 读取节点绑定的云端共签方；未绑定返回 (nil, false)。
func (k Keeper) GetCloudSigner(ctx sdk.Context, node string) (*CloudSigner, bool) {
	bz := ctx.KVStore(k.storeKey).Get(cloudSignerKey(node))
	if bz == nil {
		return nil, false
	}
	var cs CloudSigner
	if err := json.Unmarshal(bz, &cs); err != nil {
		return nil, false
	}
	return &cs, true
}

// SubmitCosign 提交云端共签并链上校验云端签名。
// payloadHash 为 hex 编码的 32 字节载荷哈希；cloudSig 为 hex 编码的 secp256k1 签名
// （对 payloadHash 原始字节）。校验通过则持久化共签证明并发出事件。
func (k Keeper) SubmitCosign(ctx sdk.Context, node, payloadHash, cloudSig string) error {
	cs, ok := k.GetCloudSigner(ctx, node)
	if !ok {
		return types.ErrNoCloudSigner.Wrap(node)
	}
	pubBytes, err := hex.DecodeString(cs.CloudPubKey)
	if err != nil || len(pubBytes) != 33 {
		return types.ErrInvalidPubKey.Wrap("stored cloud pubkey invalid")
	}
	pub := &secp256k1.PubKey{Key: pubBytes}

	payload, err := hex.DecodeString(payloadHash)
	if err != nil || len(payload) != 32 {
		return types.ErrInvalidProof.Wrap("payload_hash must be 32-byte hex")
	}
	sig, err := hex.DecodeString(cloudSig)
	if err != nil {
		return types.ErrInvalidProof.Wrap("cloud_sig must be hex")
	}
	if !pub.VerifySignature(payload, sig) {
		return types.ErrInvalidProof.Wrap("cloud signature verification failed")
	}
	att := CosignAttestation{
		Node:        node,
		PayloadHash: payloadHash,
		CloudSig:    cloudSig,
		BlockHeight: ctx.BlockHeight(),
	}
	bz, err := json.Marshal(att)
	if err != nil {
		return fmt.Errorf("phonenode: marshal cosign attestation: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(cosignKey(node), bz)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.CosignVerified",
		sdk.NewAttribute("node", node),
		sdk.NewAttribute("payload_hash", payloadHash),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
	))
	return nil
}

// GetCosign 读取节点最近一次共签证明；不存在返回 (nil, false)。
func (k Keeper) GetCosign(ctx sdk.Context, node string) (*CosignAttestation, bool) {
	bz := ctx.KVStore(k.storeKey).Get(cosignKey(node))
	if bz == nil {
		return nil, false
	}
	var att CosignAttestation
	if err := json.Unmarshal(bz, &att); err != nil {
		return nil, false
	}
	return &att, true
}
