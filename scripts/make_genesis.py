#!/usr/bin/env python3
# MC 公链 - 生产 genesis 生成器
# 作用：把一个基础 genesis.json 规范化为生产就绪（umc denom、DePIN 初始池、代币上限），
#       并断言关键账户已存在。路径无关，可在新机器复用。
# 用法：
#   python make_genesis.py --genesis <base.json> --out <prod.json> --config <genesis-config.json>
import json, sys, argparse

# ── 链上常量镜像（与 x/tokenomics/types/keys.go、x/depin/types/params.go 一一对应）──
# 任一处改动必须同步本文件，否则 CHECK_INVARIANTS 会 fail-fast 阻断创世生成。
TOTAL_SUPPLY_CAP = 1_000_000_000_000_000        # 1e15 umc = 10 亿 MC（硬顶）
DEPIN_INITIAL_POOL_SLICE = 550_000_000_000_000  # 设备激励池 55% 切片
REFERRAL_ECOSYSTEM_BUDGET = 82_500_000_000_000  # 推荐生态预算（= 55% 的 15%）
DEPIN_DEFAULT_INITIAL_POOL = DEPIN_INITIAL_POOL_SLICE - REFERRAL_ECOSYSTEM_BUDGET  # 4.675e14

# 五池分配 bps（合计 10000）：设备激励 55% / 质押安全 15% / 基金会 13% / 团队 12% / 早期开发 5%
POOL_BPS = {
    "device_incentive": 5500,
    "staking_security": 1500,
    "foundation": 1300,
    "team": 1200,
    "early_dev": 500,
}

# ── 节点治理参数：显式钉住，不隐式沿用 SDK / CometBFT 默认 ──
#
# 为什么必须显式：SDK 的默认值是按「比特币式长块时」的直觉给的，本链出块 4s，
# 两者的时间尺度差两个数量级。以 slashing 为例，默认 signed_blocks_window=100
# 块在 4s 链上只有 **400 秒**，而默认 downtime_jail_duration=600 秒——
# 漏签统计窗口比 jail 时长还短，验证人一次例行重启就会被判 downtime 并 jailed。
# 这类「隐式默认」的风险不止于此：SDK 升级可能静默改动默认值，而链上参数
# 在创世即固化，事后只能靠治理提案修正。
#
# 因此策略是：**值取当前 SDK/CometBFT 等价默认，但显式写入创世**——除了
# signed_blocks_window 必须按 4s 出块重新校准（标 ★）。这样既不引入未经论证的
# 经济改动，又消除了「默认值漂移」与「治理意图不明」两类风险。
#
# 类型约定（必须与 SDK 的 genesis JSON 编码一致，否则 InitChain 解析失败）：
#   staking.params：max_validators / max_entries / historical_entries 为 number，
#                   unbonding_time / min_commission_rate 为 string，
#   slashing.params：signed_blocks_window 为 string（proto int64 走 json 字符串），
#                   其余 Dec 亦为 string。
DEFAULT_STAKING_PARAMS = {
    "max_validators": 100,                              # SDK 默认（初始上限，治理可扩容）
    "max_entries": 7,                                   # SDK 默认
    "historical_entries": 10000,                        # SDK 默认
    "unbonding_time": "1814400s",                       # 21 天，SDK 默认
    "min_commission_rate": "0.000000000000000000",      # SDK 默认
}

DEFAULT_SLASHING_PARAMS = {
    "signed_blocks_window": "21600",                    # ★ 1 天 @4s（SDK 默认 100 块 = 400s）
    "min_signed_per_window": "0.500000000000000000",    # SDK 默认（窗口内须签满 50%）
    "downtime_jail_duration": "600s",                   # SDK 默认（10 分钟）
    "slash_fraction_double_sign": "0.050000000000000000",  # SDK 默认（5%）
    "slash_fraction_downtime": "0.010000000000000000",     # SDK 默认（1%）
}

# consensus（CometBFT）层参数默认值，与 CometBFT v0.37 默认一致。
DEFAULT_CONSENSUS_PARAMS = {
    "block_max_gas": "100000000",                       # 1e8，链上 fail-closed 钳制同值
    "block_max_bytes": "22020096",
    "block_time_iota_ms": "1000",
    "evidence_max_age_num_blocks": "100000",
    "evidence_max_age_duration": "172800000000000",     # 48h
    "evidence_max_bytes": "1048576",
}

# 出块间隔（秒）。链上 timeout_commit 固化为 4s，用于把块数参数换算成时长做交叉校验。
BLOCK_TIME_SECONDS = 4

# 链上每区块 gas 硬顶（app.DefaultBlockMaxGas）。创世写超过此值会被 app 静默钳制，
# 造成「创世参数」与「实际生效参数」不一致，故在生成阶段就拦住。
CHAIN_BLOCK_MAX_GAS = 100_000_000


def _die(msg):
    sys.stderr.write("[FATAL] %s\n" % msg)
    sys.exit(1)


def check_invariants(as_, cap, depin_pool):
    """创世不变量前置校验（对齐 x/tokenomics/keeper/genesis.go 的链上校验）。

    在生成阶段 fail-fast，避免主网 InitChain 才 panic 导致启动失败。
    """
    # I1: 总量上限必须等于链上硬编码 cap
    if int(cap) != TOTAL_SUPPLY_CAP:
        _die("tokenomics_cap=%s != 链上 TotalSupplyCap=%d（1e15 umc）" % (cap, TOTAL_SUPPLY_CAP))

    # I2: DePIN 初始池必须等于「设备激励切片 − 推荐生态预算」
    if int(depin_pool) != DEPIN_DEFAULT_INITIAL_POOL:
        _die("depin_initial_pool=%s != DepinInitialPoolSlice(%d) - ReferralEcosystemBudget(%d) = %d"
             % (depin_pool, DEPIN_INITIAL_POOL_SLICE, REFERRAL_ECOSYSTEM_BUDGET,
                DEPIN_DEFAULT_INITIAL_POOL))

    tk = as_.get("tokenomics")
    if not tk:
        return
    allocs = tk.get("allocations") or []
    if not allocs:
        _die("tokenomics.allocations 为空——五池未分配，链启动后资金无处可去")

    by_name = {}
    for a in allocs:
        name = a.get("name")
        if name in by_name:
            _die("tokenomics.allocations 出现重名分配: %s" % name)
        by_name[name] = a

    # I3: 五池齐备，且每池金额 == cap * bps / 10000
    total = 0
    for name, bps in POOL_BPS.items():
        a = by_name.get(name)
        if a is None:
            _die("tokenomics.allocations 缺失池: %s" % name)
        want = TOTAL_SUPPLY_CAP * bps // 10000
        # 字段名以链上 proto 为准：tokenomics PoolAllocation.allocated_amount
        # （proto/mcchain/tokenomics/genesis.proto:15）。
        # 读 a["amount"] 会因该键在创世 JSON 里不存在而每池读到 0 并立即 _die，
        # 使整条建链路径失效；测试夹具若同样用 "amount"，校验的就是不存在的
        # schema，给不出任何保护。这里保留 "amount" 作为兼容兜底。
        got = int(a.get("allocated_amount", a.get("amount", 0)))
        if got != want:
            _die("分配 %s = %d，应为 cap*%dbps/10000 = %d" % (name, got, bps, want))
        if not a.get("address"):
            _die("分配 %s 缺少 address（不可为占位空值）" % name)
        total += got

    # I4: 五池之和恰好等于 cap（无遗漏、无超发）
    if total != TOTAL_SUPPLY_CAP:
        _die("五池合计 %d != TotalSupplyCap %d" % (total, TOTAL_SUPPLY_CAP))

    # I5: 出现非五池的额外分配 → 会打破 I4，直接拒绝
    extra = set(by_name) - set(POOL_BPS)
    if extra:
        _die("tokenomics.allocations 含未知分配 %s（会破坏总量不变量）" % sorted(extra))


def _duration_seconds(s):
    """把 Go duration 字符串（如 1814400s / 48h / 600s）解析为秒；失败返回 None。"""
    text = str(s).strip()
    units = {"s": 1, "m": 60, "h": 3600, "d": 86400}
    if text and text[-1] in units:
        try:
            return int(float(text[:-1]) * units[text[-1]])
        except ValueError:
            return None
    return None


def apply_node_params(as_, cfg):
    """显式固化 staking / slashing 参数。

    config 中的扁平键（如 staking_max_validators）可逐项覆盖；
    未列出的项写入等价于 SDK 当前默认的值，从而把「隐式默认」变成「显式声明」。
    """
    for section, defaults in (("staking", DEFAULT_STAKING_PARAMS),
                              ("slashing", DEFAULT_SLASHING_PARAMS)):
        node = as_.get(section)
        if not node:
            continue
        params = node.get("params") or {}
        for key, default in defaults.items():
            flat = section + "_" + key
            params[key] = cfg[flat] if flat in cfg else default
        node["params"] = params
        as_[section] = node


def check_node_params(as_):
    """节点治理参数校验 + 跨参数交叉校验（fail-fast）。

    重点不是「值是否等于某个数」，而是「这组值在 4s 出块下是否自洽」——
    单看每个参数都落在 SDK 合法区间内，组合起来仍可能让验证人不可运维。
    """
    sp = (as_.get("staking") or {}).get("params") or {}
    slp = (as_.get("slashing") or {}).get("params") or {}

    if sp:
        mv = int(sp.get("max_validators", 0))
        if not (1 <= mv <= 1000):
            _die("staking.max_validators=%s 越界（应在 1..1000）" % mv)
        ub = _duration_seconds(sp.get("unbonding_time", ""))
        if ub is None:
            _die("staking.unbonding_time=%r 不是合法的 Go duration 字符串"
                 % sp.get("unbonding_time"))
        if ub < 86400:
            _die("staking.unbonding_time=%ds 过短：解绑期是安全阀，"
                 "低于 1 天会显著降低作恶成本" % ub)

    if slp:
        win = int(slp.get("signed_blocks_window", 0))
        if win <= 0:
            _die("slashing.signed_blocks_window=%s 必须为正" % win)
        msp = float(slp.get("min_signed_per_window", 0))
        if not (0 < msp <= 1):
            _die("slashing.min_signed_per_window=%s 越界（应在 (0,1]）" % msp)
        jail = _duration_seconds(slp.get("downtime_jail_duration", ""))
        if jail is None:
            _die("slashing.downtime_jail_duration=%r 不是合法的 Go duration 字符串"
                 % slp.get("downtime_jail_duration"))
        for key in ("slash_fraction_double_sign", "slash_fraction_downtime"):
            frac = float(slp.get(key, 0))
            if not (0 <= frac <= 1):
                _die("slashing.%s=%s 越界（应在 [0,1]）" % (key, frac))

        # ★ 交叉校验：漏签统计窗口的**时长**必须显著大于 jail 时长。
        #    SDK 默认（100 块 / 600s）在 4s 链上只有 400s 窗口，比 jail 本身还短，
        #    意味着「窗口内必然有相当比例的区块没签」，验证人一次例行重启即被惩罚。
        #    这里要求窗口时长 >= 10 × jail 时长，给运维留出可恢复的余量。
        win_secs = win * BLOCK_TIME_SECONDS
        if win_secs < 10 * jail:
            _die("slashing 参数在 4s 出块下不自洽：signed_blocks_window=%d 块 × %ds = %ds，"
                 "而 downtime_jail_duration=%ds —— 窗口时长不足 jail 的 10 倍，"
                 "验证人例行维护即可能被 jail。请按出块节奏放大窗口。"
                 % (win, BLOCK_TIME_SECONDS, win_secs, jail))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--genesis", required=True, help="基础 genesis.json（mcchaind init 产物）")
    ap.add_argument("--out", required=True, help="输出生产 genesis.json 路径")
    ap.add_argument("--config", required=True, help="genesis-config.json（见 genesis-config.example.json）")
    args = ap.parse_args()

    g = json.load(open(args.genesis, encoding="utf-8"))
    cfg = json.load(open(args.config, encoding="utf-8"))
    as_ = g["app_state"]
    denom = cfg.get("bond_denom", "umc")

    # 1) denom 规范化（staking / mint / gov / crisis）
    as_["staking"]["params"]["bond_denom"] = denom
    as_["mint"]["params"]["mint_denom"] = denom
    # 固定总量链——mint 模块默认 inflation≈13% 且持有 Minter，
    # 会绕过 tokenomics 的 cap 直接二次通胀。强制清零（app.InitChainer 亦兜底）。
    # 注意：goal_bonded 绝不可归零——mint.BeginBlock 会算 bondedRatio/goal_bonded，
    # 归零将在首区块除零 panic 导致链 halt。仅清零通胀上下限 + Minter 通胀/年拨付。
    ZERO = "0.000000000000000000"
    mp = as_["mint"]["params"]
    for k in ("inflation_rate_change", "inflation_max", "inflation_min"):
        if k in mp:
            mp[k] = ZERO
    as_["mint"]["params"] = mp
    mtr = as_["mint"].get("minter", {})
    if mtr:
        mtr["inflation"] = ZERO
        mtr["annual_provisions"] = ZERO
        as_["mint"]["minter"] = mtr
    for d in (as_.get("gov", {}).get("params", {}) or {}).get("min_deposit", []) or []:
        if d.get("denom") in ("stake",):
            d["denom"] = denom
    cf = (as_.get("crisis", {}) or {}).get("constant_fee")
    if cf and cf.get("denom") in ("stake",):
        cf["denom"] = denom

    # 1.5) 治理参数规范化（DAO 开箱可用）
    gov = as_.get("gov", {})
    gp = gov.get("params", {})
    if gp:
        # min_deposit denom 强制 umc（兼容旧 stake）
        md = gp.get("min_deposit", []) or []
        for d in md:
            if d.get("denom") in ("stake",):
                d["denom"] = denom
        if md:
            d0 = cfg.get("gov_min_deposit_amount")
            if d0 is not None:
                md[0]["amount"] = str(d0)
        gp["voting_period"] = cfg.get("gov_voting_period", "172800s")
        gp["max_deposit_period"] = cfg.get("gov_max_deposit_period", "172800s")
        gp["quorum"] = cfg.get("gov_quorum", "0.334000000000000000")
        gp["threshold"] = cfg.get("gov_threshold", "0.500000000000000000")
        gp["veto_threshold"] = cfg.get("gov_veto_threshold", "0.334000000000000000")
        gov["params"] = gp
        as_["gov"] = gov

    # 1.6) consensus_params 显式固化（兜底）
    # 出块时间由 CometBFT config.toml 的 timeout_commit 控制（部署脚本统一写为 4s），
    # 本步把「每区块 gas 上限」在 genesis 中
    # 显式写为 1e8，杜绝默认 -1（不限）导致单笔 tx 占满区块、出块失控的 DoS。
    #
    # 注意：不能用 `if not blk.get("max_gas")` 判断——CometBFT 生成的默认值是
    # 字符串 "-1"（非空字符串为真值），条件恒不成立，创世里就会始终留着「不限
    # gas」。虽然 app 层有 DefaultBlockMaxGas 兜底钳制，但创世与注释口径不一致
    # 本身就是隐患（外部审计按创世读参数会得出错误结论）。故一律强制写入。
    cp = g.get("consensus_params") or {}
    blk = cp.get("block") or {}
    dp = DEFAULT_CONSENSUS_PARAMS
    # gas 上限的优先级：config 显式值 > 基础 genesis 中的合法正值 > 链上硬顶默认值。
    # 中间这一层是为了不破坏 deploy/init.sh 的 MAX_GAS 覆盖链路——它会把 MAX_GAS
    # 直接写进基础 genesis 的 consensus_params.block.max_gas；只有当基础 genesis
    # 的值缺失或非正数（如 CometBFT 默认的 "-1"）时才兜底为链上硬顶。
    existing_gas = blk.get("max_gas")
    try:
        existing_gas_ok = int(existing_gas) > 0
    except (TypeError, ValueError):
        existing_gas_ok = False
    blk["max_gas"] = str(cfg.get(
        "block_max_gas", existing_gas if existing_gas_ok else dp["block_max_gas"]))
    blk["max_bytes"] = str(cfg.get("block_max_bytes", dp["block_max_bytes"]))
    blk["time_iota_ms"] = str(cfg.get("block_time_iota_ms", dp["block_time_iota_ms"]))
    # 兼容旧字段名：CometBFT v0.37 已把 max_evidence_age 从 block 迁到 evidence。
    blk.pop("max_evidence_age", None)
    cp["block"] = blk
    ev = cp.get("evidence") or {}
    ev["max_age_num_blocks"] = str(
        cfg.get("evidence_max_age_num_blocks", dp["evidence_max_age_num_blocks"]))
    ev["max_age_duration"] = str(
        cfg.get("evidence_max_age_duration", dp["evidence_max_age_duration"]))
    ev["max_bytes"] = str(cfg.get("evidence_max_bytes", dp["evidence_max_bytes"]))
    cp["evidence"] = ev
    cp.setdefault("validator", {"pub_key_types": ["ed25519"]})
    cp.setdefault("version", {"app_version": "1"})
    g["consensus_params"] = cp

    # 1.6.1) gas 上限自校验：创世值必须为正且不超过链上硬顶，
    # 否则「创世参数」与「实际生效参数」不一致（app 层会静默钳制）。
    try:
        gas = int(blk["max_gas"])
    except (TypeError, ValueError):
        _die("consensus block.max_gas=%r 必须为整数" % blk["max_gas"])
    if gas <= 0:
        _die("consensus block.max_gas=%s 必须为正（-1 表示不限，"
             "会打开单笔 tx 占满区块的 DoS 面）" % gas)
    if gas > CHAIN_BLOCK_MAX_GAS:
        _die("consensus block.max_gas=%d 超过链上硬顶 %d（app.DefaultBlockMaxGas），"
             "创世值会被静默钳制，请改为不超过硬顶的值" % (gas, CHAIN_BLOCK_MAX_GAS))

    # 1.7) staking / slashing 参数显式固化
    # 不再隐式沿用 SDK 默认：默认值按长块时直觉给出，与本链 4s 出块差两个数量级。
    # 详见文件顶部 DEFAULT_STAKING_PARAMS / DEFAULT_SLASHING_PARAMS 的说明。
    apply_node_params(as_, cfg)
    check_node_params(as_)

    # 2) DePIN 初始池 + 奖励 denom
    # 兜底默认值必须等于链上 depin.DefaultInitialPool，否则 config 漏项会静默写入
    # 错误池额，直到 InitChain 才因不变量校验失败而 panic。
    depin_pool = int(cfg.get("depin_initial_pool", DEPIN_DEFAULT_INITIAL_POOL))
    if "depin" in as_:
        as_["depin"]["params"]["reward_denom"] = denom
        as_["depin"]["params"]["initial_pool"] = str(depin_pool)

    # 3) tokenomics 上限 + denom（结构: tokenomics.{denom,total_supply_cap,allocations,release}）
    tk_cap = int(cfg.get("tokenomics_cap", TOTAL_SUPPLY_CAP))
    if "tokenomics" in as_:
        as_["tokenomics"]["denom"] = denom
        if "total_supply_cap" in as_["tokenomics"]:
            as_["tokenomics"]["total_supply_cap"] = tk_cap

    # 3.5) edgeai 仲裁者地址：默认取 tokenomics 的「团队」分配地址
    # （权威来源，与链上 TeamAddress 由同一组多签公钥推导，必然一致），
    # config 的 edgeai_arbitrator 仅作兜底/覆盖。绝不接受手填的非法 bech32，
    # 否则 InitChain 会因 SetParams 校验失败而 panic。
    if "edgeai" in as_ and "tokenomics" in as_:
        team_addr = None
        for a in as_["tokenomics"].get("allocations", []) or []:
            if a.get("name") == "team":
                team_addr = a.get("address")
                break
        arb = team_addr or cfg.get("edgeai_arbitrator")
        if arb:
            as_["edgeai"]["params"]["arbitrator"] = arb

    # 4) 断言关键账户已存在（防止漏加 genesis 账户）
    # 主网 chain-id 下 assert_accounts 必须全部存在，缺失即
    # fail-fast 阻断 genesis 生成；非主网（dev/test/local/sim）降级为 WARN，保留
    # 本地启动与 CI 能力。这把「团队/基金会/早期开发/仲裁者等核心账户漏配」从
    # 「静默通过、出块后资金无处可去」变为「生成阶段强制拦截」。
    is_mainnet = "mainnet" in str(g.get("chain_id", "")).lower()
    accs = (as_.get("auth", {}) or {}).get("accounts", []) or []
    existing = {a.get("address") for a in accs}
    for need in cfg.get("assert_accounts", []) or []:
        if need not in existing:
            if is_mainnet:
                _die("assert_accounts 缺失（主网禁止）: %s —— 必须在 genesis 中配置真实核心账户" % need)
            sys.stderr.write("[WARN] assert_accounts 缺失: %s\n" % need)

    # 4.5) edgeai 仲裁者：主网下必须显式配置多签/真实治理地址，
    # 缺失即 fail-fast；非主网回退 tokenomics 的 team 分配地址（见 3.5）。
    if "edgeai" in as_:
        arb = as_["edgeai"].get("params", {}).get("arbitrator")
        if not arb:
            if is_mainnet:
                _die("edgeai.arbitrator 缺失（主网禁止）——必须配置治理多签地址")
            sys.stderr.write("[WARN] edgeai.arbitrator 缺失，将回退 tokenomics team 地址\n")

    # 5) chain_id
    if cfg.get("chain_id"):
        g["chain_id"] = cfg["chain_id"]

    # 6) 不变量前置校验（fail-fast，绝不产出一个会让主网 InitChain panic 的 genesis）
    check_invariants(as_, tk_cap, depin_pool)

    # 6.5) 创世账本总供应不变量
    #
    # 为什么必须在这里拦：链上 app.assertGenesisSupplyCap 只在 InitChain 末尾核对
    # 真实账本总供应与 1e15 硬顶，生产链超标直接拒绝启动 —— 但那时已经打完 gentx、
    # 公示过 genesis sha256、通知过验证人。本检查把失败提前到 genesis 构建阶段。
    #
    # 口径：
    #   · 全新创世（tokenomics.minted_supply == 0）：tokenomics 在 InitGenesis 一次性
    #     铸造 1e15 并拨入五池，因此创世 bank.balances **必须为空**。任何余额都来自
    #     add-genesis-account（凭空记账），必然击穿「总量 10 亿 MC 恒定」。
    #     主网验证人的自抵押资金来自五池中的团队多签 vesting 账户，不需要（也不允许）
    #     add-genesis-account —— gentx 只需生成期临时账户，生成后必须删除。
    #   · 恢复创世（export 导出，minted_supply > 0）：代币早已在源链铸好并随 auth/bank
    #     原样带入，此时总供应必须**恰好等于**硬顶。
    _bank_total = sum(int(b.get("amount", 0))
                      for b in (((as_.get("bank") or {}).get("balances")) or []))
    _minted = int((as_.get("tokenomics") or {}).get("minted_supply", 0) or 0)
    if _minted > 0:
        if _bank_total != TOTAL_SUPPLY_CAP:
            _die("恢复创世（minted_supply=%d）账本总供应 %d != 硬顶 %d —— "
                 "export/import 往返丢了或多了余额" % (_minted, _bank_total, TOTAL_SUPPLY_CAP))
    elif _bank_total != 0:
        n = len((((as_.get("bank") or {}).get("balances")) or []))
        _die("全新创世的 bank 余额必须为 0，实际 %d umc（%d 个账户）—— "
             "出现余额说明用了 add-genesis-account（凭空记账）。"
             "主网验证人自抵押资金必须来自五池的团队多签 vesting 账户；"
             "gentx 的临时 genesis-account 必须在 collect-gentxs 之后删除。"
             % (_bank_total, n))

    json.dump(g, open(args.out, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
    print("PROD GENESIS OK -> %s" % args.out)
    print("  ledger supply     = %d umc（%s）"
          % (_bank_total, "恢复创世 = 硬顶" if _minted > 0 else "全新创世 = 0（tokenomics 创世时铸 1e15）"))
    print("  bond_denom        = %s" % as_["staking"]["params"]["bond_denom"])
    print("  mint_denom        = %s" % as_["mint"]["params"]["mint_denom"])
    # 守卫：depin / tokenomics 可能不在 app_state 中，缺失时给出明确报错
    if "depin" in as_:
        print("  depin.initial_pool= %s umc" % as_["depin"]["params"]["initial_pool"])
    if "tokenomics" in as_:
        print("  tokenomics.cap    = %s umc" % as_["tokenomics"].get("total_supply_cap"))
    print("  gov.voting_period = %s" % gp.get("voting_period"))
    print("  gov.quorum/threshold/veto = %s / %s / %s" % (
        gp.get("quorum"), gp.get("threshold"), gp.get("veto_threshold")))
    stp = (as_.get("staking") or {}).get("params") or {}
    slp = (as_.get("slashing") or {}).get("params") or {}
    if stp:
        print("  staking.max_validators = %s（治理可扩容）" % stp.get("max_validators"))
        print("  staking.unbonding_time = %s" % stp.get("unbonding_time"))
    if slp:
        win_blocks = int(slp.get("signed_blocks_window", 0))
        print("  slashing.window   = %s 块 = %s 秒 @%ds（min_signed=%s）" % (
            slp.get("signed_blocks_window"), win_blocks * BLOCK_TIME_SECONDS,
            BLOCK_TIME_SECONDS, slp.get("min_signed_per_window")))
    print("  block.max_gas     = %s（链上硬顶 %d）" % (blk.get("max_gas"), CHAIN_BLOCK_MAX_GAS))
    print("  chain_id          = %s" % g["chain_id"])


if __name__ == "__main__":
    main()
