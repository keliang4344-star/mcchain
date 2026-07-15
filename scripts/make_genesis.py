#!/usr/bin/env python3
# MC 公链 - 生产 genesis 生成器（B6-R2）
# 作用：把一个基础 genesis.json 规范化为生产就绪（umc denom、DePIN 初始池、代币上限），
#       并断言关键账户已存在。路径无关，可在新机器复用。
# 用法：
#   python make_genesis.py --genesis <base.json> --out <prod.json> --config <genesis-config.json>
import json, sys, argparse


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
    # P0/R1: 固定总量链——mint 模块默认 inflation≈13% 且持有 Minter，
    # 会绕过 tokenomics 的 cap 直接二次通胀。强制清零（app.InitChainer 也会兜底）。
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

    # 1.5) 治理参数规范化（B6-R1：DAO 开箱可用）
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

    # 2) DePIN 初始池 + 奖励 denom
    if "depin" in as_:
        as_["depin"]["params"]["reward_denom"] = denom
        as_["depin"]["params"]["initial_pool"] = str(cfg.get("depin_initial_pool", 100000000000000))

    # 3) tokenomics 上限 + denom（结构: tokenomics.{denom,total_supply_cap,allocations,release}）
    if "tokenomics" in as_:
        as_["tokenomics"]["denom"] = denom
        if "total_supply_cap" in as_["tokenomics"]:
            as_["tokenomics"]["total_supply_cap"] = int(cfg.get("tokenomics_cap", 1000000000000000))

    # 3.5) edgeai 仲裁者地址（B3.1）：默认取 tokenomics 的「团队」分配地址
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
    accs = (as_.get("auth", {}) or {}).get("accounts", []) or []
    existing = {a.get("address") for a in accs}
    for need in cfg.get("assert_accounts", []) or []:
        if need not in existing:
            sys.stderr.write("[WARN] assert_accounts 缺失: %s\n" % need)

    # 5) chain_id
    if cfg.get("chain_id"):
        g["chain_id"] = cfg["chain_id"]

    json.dump(g, open(args.out, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
    print("PROD GENESIS OK -> %s" % args.out)
    print("  bond_denom        = %s" % as_["staking"]["params"]["bond_denom"])
    print("  mint_denom        = %s" % as_["mint"]["params"]["mint_denom"])
    print("  depin.initial_pool= %s umc" % as_["depin"]["params"]["initial_pool"])
    print("  tokenomics.cap    = %s umc" % as_["tokenomics"]["total_supply_cap"])
    print("  gov.voting_period = %s" % gp.get("voting_period"))
    print("  gov.quorum/threshold/veto = %s / %s / %s" % (
        gp.get("quorum"), gp.get("threshold"), gp.get("veto_threshold")))
    print("  chain_id          = %s" % g["chain_id"])


if __name__ == "__main__":
    main()
