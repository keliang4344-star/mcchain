package app

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// MinSelfDelegationLowerBound is the chain-wide minimum self delegation enforced
// for every validator, expressed in the base denom (umc).
//
//	3e10 umc == 30_000 MC == 30k MC
//
// It is the single source of truth shared by the ante decorator (transaction
// path) and the InitChainer fallback (genesis validators, which bypass the ante
// chain).
const MinSelfDelegationLowerBound = 30_000_000_000 // umc = 30k MC

// maxAnteUnwrapDepth 限制了嵌套消息的解包深度。
//
// 正常交易最多出现一层包裹（authz.MsgExec / gov / group 提案各一层），
// MsgExec 内再套 MsgExec 在实践中不存在。设上限是为了让「构造任意深度嵌套以
// 绕过检查」在 gas 耗尽之前就被拒绝，而不是变成一个无界递归。
const maxAnteUnwrapDepth = 4

// nestedMsgProvider 是所有「内部还包着别的消息」的消息类型的公共形态。
//
//	*authz.MsgExec           —— 授权代执行（主要的绕过面）
//	*govv1.MsgSubmitProposal —— 治理提案携带的消息
//	*group.MsgSubmitProposal —— 群组提案携带的消息
//
// 三者都实现了 GetMessages() ([]sdk.Msg, error)，因此这里只依赖这一个方法，
// 不为每个具体类型写 case —— 将来新增同类容器消息会自动被覆盖，不会因为
// 「忘了加一个 case」而重新打开绕过口子。
type nestedMsgProvider interface {
	GetMessages() ([]sdk.Msg, error)
}

// MinSelfDelegationDecorator enforces a global minimum self delegation for
// validators. Cosmos SDK v0.47 has no built-in global floor, so we inspect
// every transaction's messages and reject MsgCreateValidator / MsgEditValidator
// whose MinSelfDelegation is below the chain-wide bound.
//
// Genesis validators are created directly by InitGenesis (not via tx) and are
// therefore handled separately in App.InitChainer.
//
// # 嵌套解包
//
// 朴素实现只对 tx.GetMsgs 做顶层类型断言：
//
//	for _, msg := range tx.GetMsgs() {
//	    switch m := msg.(type) {
//	    case *stakingtypes.MsgCreateValidator: ...
//
// 而本链启用了 authz 与 feegrant，于是任何人只要发一笔 authz.MsgExec 包裹
// MsgCreateValidator，顶层看到的是 *authz.MsgExec，落不到任何 case 分支，直接
// 放行；随后 authz 的 msg server 会把内层消息解出来真正执行。等价地，治理提案
// 与群组提案携带的消息同样不经过这一层判定（由各自模块在提案通过后执行）。
//
// 结果：白皮书 §A.6 承诺的「验证人自抵押 ≥3 万 MC」这条链级底线被完全绕过，
// 验证人可以以 1 umc 自抵押进入集合 —— 「利益绑定」的经济前提不复存在。
//
// 现改为递归解包：对任何 nestedMsgProvider 取出其内部消息，连同顶层一起逐层
// 检查，并设深度上限。解包失败一律 fail-closed —— 无法确认内层内容时放行，
// 等于把绕过口子重新打开。
type MinSelfDelegationDecorator struct {
	// registry 用于在 GetMessages() 缓存未填充时自行解包 Any。
	// 允许为 nil（单测直接构造零值时）；此时遇到嵌套消息会 fail-closed。
	registry codectypes.InterfaceRegistry
}

var _ sdk.AnteDecorator = MinSelfDelegationDecorator{}

// NewMinSelfDelegationDecorator 构造装饰器，registry 由 app 注入。
func NewMinSelfDelegationDecorator(registry codectypes.InterfaceRegistry) MinSelfDelegationDecorator {
	return MinSelfDelegationDecorator{registry: registry}
}

// AnteHandle implements sdk.AnteDecorator.
func (msd MinSelfDelegationDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	if err := msd.checkMsgs(tx.GetMsgs(), 0); err != nil {
		return ctx, err
	}
	return next(ctx, tx, simulate)
}

// checkMsgs 逐条校验消息，并递归解包容器类消息（authz / gov / group）。
func (msd MinSelfDelegationDecorator) checkMsgs(msgs []sdk.Msg, depth int) error {
	if depth > maxAnteUnwrapDepth {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
			"message nesting deeper than %d levels is not allowed", maxAnteUnwrapDepth)
	}

	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		// ---- 1. 叶子校验：本链关心的只有验证人自抵押下限 ----
		switch m := msg.(type) {
		case *stakingtypes.MsgCreateValidator:
			if m.MinSelfDelegation.LT(sdk.NewInt(MinSelfDelegationLowerBound)) {
				return sdkerrors.Wrapf(
					sdkerrors.ErrInvalidRequest,
					"min self delegation %s < lower bound %d umc",
					m.MinSelfDelegation.String(),
					MinSelfDelegationLowerBound,
				)
			}
		case *stakingtypes.MsgEditValidator:
			// A zero/min-self-delegation value means "do not change"; the field
			// is only validated when it is actually set (non-nil pointer and non-nil underlying).
			// 注意：m.MinSelfDelegation 是 *sdk.Int，nil 指针时不能直接调 .IsNil()（会解引用 panic）。
			if m.MinSelfDelegation != nil && !m.MinSelfDelegation.IsNil() && m.MinSelfDelegation.IsPositive() {
				if m.MinSelfDelegation.LT(sdk.NewInt(MinSelfDelegationLowerBound)) {
					return sdkerrors.Wrapf(
						sdkerrors.ErrInvalidRequest,
						"min self delegation %s < lower bound %d umc",
						m.MinSelfDelegation.String(),
						MinSelfDelegationLowerBound,
					)
				}
			}
		}

		// ---- 2. 容器校验：递归进入内层消息 ----
		provider, ok := msg.(nestedMsgProvider)
		if !ok {
			continue
		}

		inner, err := provider.GetMessages()
		if err != nil {
			// GetMessages() 读的是解码时填好的缓存；缓存缺失时自行解包一次再取。
			if unpackErr := msd.unpack(msg); unpackErr != nil {
				return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
					"cannot inspect nested messages of %T: %v", msg, unpackErr)
			}
			inner, err = provider.GetMessages()
			if err != nil {
				return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
					"cannot inspect nested messages of %T: %v", msg, err)
			}
		}

		if err := msd.checkMsgs(inner, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// unpack 用 InterfaceRegistry 解开消息内嵌的 Any，填充 GetMessages() 依赖的缓存。
//
// 失败时返回错误而不是忽略：调用方据此 fail-closed。宁可拒绝一笔无法检查的
// 交易，也不能放行一笔无法检查的交易 —— 后者正是本处拆解要消除的东西。
func (msd MinSelfDelegationDecorator) unpack(msg sdk.Msg) error {
	if msd.registry == nil {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest,
			"interface registry unavailable; refusing to execute a msg whose nested messages cannot be inspected")
	}
	return codectypes.UnpackInterfaces(msg, msd.registry)
}
