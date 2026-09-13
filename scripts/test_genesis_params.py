#!/usr/bin/env python3
"""创世参数固化回归测试。

验证 make_genesis.py：
  1. 把 staking / slashing / consensus 参数显式写入创世（不再隐式沿用 SDK 默认）；
  2. 把 CometBFT 默认的 max_gas="-1" **真正覆盖**为链上硬顶——若写作
     `if not blk.get("max_gas")`，"-1" 是非空字符串（真值），条件恒不成立，
     创世里就会始终留着「不限 gas」；
  3. signed_blocks_window 按 4s 出块校准（默认 100 块 = 400 秒，比 jail 时长还短）；
  4. 对「4s 出块下不自洽」的 slashing 参数 fail-fast。

用法：python scripts/test_genesis_params.py     （退出码 0 = 全部通过）
"""
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
MAKE = os.path.join(HERE, "make_genesis.py")
PY = sys.executable

VALID_ARBITRATOR = "mc1psg2qr3d6vhtpcdhh56nd2wxfyted5an95s2c0"  # 链上 TeamAddress（team_pubkeys_gen.go 派生，见 scripts/verify_team_recover.sh）
POOL_ALLOCS = [
    ("device_incentive", "550000000000000"),
    ("staking_security", "150000000000000"),
    ("foundation", "130000000000000"),
    ("team", "120000000000000"),
    ("early_dev", "50000000000000"),
]

_failures = []


def check(cond, label, detail=""):
    if cond:
        print("  [PASS] %s" % label)
    else:
        print("  [FAIL] %s%s" % (label, ("  -> " + detail) if detail else ""))
        _failures.append(label)


def base_genesis():
    """模拟 mcchaind init 的产物：注意 max_gas 刻意留 CometBFT 默认的 "-1"，
    slashing 窗口刻意留 SDK 默认的 "100"，用于验证两者都会被修正。"""
    return {
        "chain_id": "mcchain-mainnet-1",
        "app_state": {
            "staking": {"params": {
                "bond_denom": "stake",
                "unbonding_time": "1814400s",
                "max_validators": 100,
                "max_entries": 7,
                "historical_entries": 10000,
                "min_commission_rate": "0.000000000000000000",
            }},
            "slashing": {"params": {
                "signed_blocks_window": "100",
                "min_signed_per_window": "0.500000000000000000",
                "downtime_jail_duration": "600s",
                "slash_fraction_double_sign": "0.050000000000000000",
                "slash_fraction_downtime": "0.010000000000000000",
            }},
            "mint": {
                "params": {"mint_denom": "stake", "goal_bonded": "0.670000000000000000",
                           "inflation_rate_change": "0.130000000000000000",
                           "inflation_max": "0.200000000000000000",
                           "inflation_min": "0.070000000000000000"},
                "minter": {"inflation": "0.130000000000000000", "annual_provisions": "0"},
            },
            "gov": {"params": {
                "min_deposit": [{"denom": "stake", "amount": "10000000"}],
                "voting_period": "172800s", "max_deposit_period": "172800s",
                "quorum": "0.334000000000000000", "threshold": "0.500000000000000000",
                "veto_threshold": "0.334000000000000000",
            }},
            "crisis": {"constant_fee": {"denom": "stake", "amount": "1000"}},
            "depin": {"params": {"reward_denom": "stake", "initial_pool": "0"}},
            "tokenomics": {
                "denom": "stake", "total_supply_cap": "0",
                # 字段名必须与链上 proto 一致：PoolAllocation.allocated_amount。
                # 夹具用 "amount" 与真实创世 JSON 的 "allocated_amount" 不符，
                # 测试就会对读错键名的缺陷完全免疫（测试绿、生产路径死）。
                # 夹具必须用真 schema。
                "allocations": [{"name": n, "allocated_amount": a,
                                 "address": "mc1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqhq4d2z"}
                                for n, a in POOL_ALLOCS],
            },
            "edgeai": {"params": {"arbitrator": VALID_ARBITRATOR}},
        },
        "consensus_params": {
            "block": {"max_bytes": "22020096", "max_gas": "-1"},
            "evidence": {"max_age_num_blocks": "100000",
                         "max_age_duration": "172800000000000", "max_bytes": "1048576"},
        },
    }


def run_genesis(genesis, config_overrides=None):
    """跑 make_genesis.py，返回 (returncode, stdout+stderr, 输出genesis或None)。"""
    config = {
        "chain_id": "mcchain-mainnet-1", "bond_denom": "umc",
        "depin_initial_pool": "467500000000000", "tokenomics_cap": "1000000000000000",
        "assert_accounts": [], "gov_min_deposit_amount": "10000000",
        "gov_voting_period": "172800s", "gov_max_deposit_period": "172800s",
        "gov_quorum": "0.334000000000000000", "gov_threshold": "0.500000000000000000",
        "gov_veto_threshold": "0.334000000000000000", "edgeai_arbitrator": VALID_ARBITRATOR,
    }
    config.update(config_overrides or {})

    with tempfile.TemporaryDirectory() as tmp:
        gpath = os.path.join(tmp, "base.json")
        cpath = os.path.join(tmp, "cfg.json")
        opath = os.path.join(tmp, "out.json")
        with open(gpath, "w", encoding="utf-8") as f:
            json.dump(genesis, f)
        with open(cpath, "w", encoding="utf-8") as f:
            json.dump(config, f)
        proc = subprocess.run([PY, MAKE, "--genesis", gpath, "--out", opath, "--config", cpath],
                              capture_output=True, text=True, encoding="utf-8")
        out = None
        if os.path.exists(opath):
            with open(opath, encoding="utf-8") as f:
                out = json.load(f)
        return proc.returncode, (proc.stdout or "") + (proc.stderr or ""), out


def test_params_are_pinned():
    print("[1] 参数显式钉入创世")
    rc, log, g = run_genesis(base_genesis())
    check(rc == 0, "生成成功", log[-400:])
    if rc != 0:
        return
    as_ = g["app_state"]

    check(g["consensus_params"]["block"]["max_gas"] == "100000000",
          "max_gas 被覆盖为 1e8（真值判断会保留 \"-1\"）",
          "got %r" % g["consensus_params"]["block"]["max_gas"])
    check(g["consensus_params"]["block"].get("max_evidence_age") is None,
          "已迁出的 max_evidence_age 被清理")

    check(as_["staking"]["params"]["bond_denom"] == "umc", "staking.bond_denom = umc")
    check(as_["staking"]["params"]["max_validators"] == 100, "staking.max_validators 显式写入")
    check(as_["staking"]["params"]["unbonding_time"] == "1814400s", "staking.unbonding_time 显式写入")
    check("min_commission_rate" in as_["staking"]["params"], "staking.min_commission_rate 显式写入")

    slp = as_["slashing"]["params"]
    check(slp["signed_blocks_window"] == "21600",
          "slashing 窗口按 4s 校准为 21600 块（= 1 天）",
          "got %r" % slp["signed_blocks_window"])
    check(slp["downtime_jail_duration"] == "600s", "downtime_jail_duration 显式写入")
    check(isinstance(slp["signed_blocks_window"], str), "signed_blocks_window 保持字符串类型")

    check(as_["mint"]["params"]["inflation_max"] == "0.000000000000000000",
          "零通胀：inflation_max 被清零")
    check(as_["mint"]["params"]["goal_bonded"] != "0.000000000000000000",
          "goal_bonded 未被清零（清零会导致首区块除零 panic）")


def test_config_override():
    print("[2] config 覆盖生效")
    rc, log, g = run_genesis(base_genesis(), {"slashing_signed_blocks_window": "43200",
                                              "staking_max_validators": 50})
    check(rc == 0, "生成成功", log[-400:])
    if rc != 0:
        return
    check(g["app_state"]["slashing"]["params"]["signed_blocks_window"] == "43200", "窗口可被 config 覆盖")
    check(g["app_state"]["staking"]["params"]["max_validators"] == 50, "验证人上限可被 config 覆盖")


def test_inconsistent_slashing_rejected():
    print("[3] 不自洽的 slashing 参数被 fail-fast 拦住")
    # 窗口时长必须 >= 10×jail 时长：600s jail → 至少 6000s → 1500 块 @4s。
    rc, log, g = run_genesis(base_genesis(), {"slashing_signed_blocks_window": "100"})
    check(rc != 0, "100 块（400s）窗口被拒绝（< 10×600s）", "rc=%d" % rc)
    check("不自洽" in log or "FATAL" in log, "给出可读的失败原因", log[-300:])


def test_bad_gas_rejected():
    print("[4] 非法 gas 上限被拒绝")
    rc, log, _ = run_genesis(base_genesis(), {"block_max_gas": "0"})
    check(rc != 0, "max_gas=0 被拒绝", "rc=%d" % rc)
    rc, log, _ = run_genesis(base_genesis(), {"block_max_gas": "200000000"})
    check(rc != 0, "max_gas 超过链上硬顶被拒绝", "rc=%d" % rc)


def test_existing_gas_preserved():
    print("[5] 基础 genesis 中的合法 gas 上限被保留（deploy/init.sh 的 MAX_GAS 链路）")
    genesis = base_genesis()
    genesis["consensus_params"]["block"]["max_gas"] = "50000000"
    rc, log, out = run_genesis(genesis)
    check(rc == 0, "生成成功", log[-400:])
    if rc != 0:
        return
    check(out["consensus_params"]["block"]["max_gas"] == "50000000",
          "已有合法值 5e7 未被兜底默认覆盖",
          "got %r" % out["consensus_params"]["block"]["max_gas"])


def main():
    print("=" * 62)
    print("创世参数固化回归测试")
    print("=" * 62)
    test_params_are_pinned()
    test_config_override()
    test_inconsistent_slashing_rejected()
    test_bad_gas_rejected()
    test_existing_gas_preserved()
    print("=" * 62)
    if _failures:
        print("FAILED: %d 项未通过 -> %s" % (len(_failures), _failures))
        return 1
    print("ALL PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
