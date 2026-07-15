import json, sys

p = r"$HOME/mcchain\testnet\config\genesis.json"
g = json.load(open(p, encoding="utf-8"))

as_ = g["app_state"]

# staking bond denom
as_["staking"]["params"]["bond_denom"] = "umc"
# mint denom
as_["mint"]["params"]["mint_denom"] = "umc"
# gov min deposit denom
if as_.get("gov") and as_["gov"].get("params") and as_["gov"]["params"].get("min_deposit"):
    for d in as_["gov"]["params"]["min_deposit"]:
        if d.get("denom") == "stake":
            d["denom"] = "umc"
# crisis constant fee denom
if as_.get("crisis") and as_["crisis"].get("constant_fee"):
    if as_["crisis"]["constant_fee"].get("denom") == "stake":
        as_["crisis"]["constant_fee"]["denom"] = "umc"

# sanity: ensure tokenomics + depin denoms are umc
assert as_["tokenomics"]["denom"] == "umc"
assert as_["depin"]["params"]["reward_denom"] == "umc"
assert as_["depin"]["params"]["initial_pool"] == "100000000000000"

json.dump(g, open(p, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
print("genesis denom fix done: bond_denom=%s mint_denom=%s" % (
    as_["staking"]["params"]["bond_denom"], as_["mint"]["params"]["mint_denom"]))
