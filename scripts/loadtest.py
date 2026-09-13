#!/usr/bin/env python3
"""
MC 公链 · 上线前压力测试工具

对应验收标准（docs/CAPACITY_PLAN.md / 第一阶段清单 M9）：
  - RPC  查询 1000 并发
  - REST 查询 200 QPS
  - 注册/API 并发 200 × 10min（对 APP 后端）
  - tx 吞吐（广播路径，验证 500 tx/s 余量）

设计约束：
  - **纯 Python 标准库**（urllib + threading + argparse + json + statistics），
    部署机上零依赖，scp 过去就能跑；
  - tx 广播模式借助 mcchaind CLI 并发子进程（签名复杂度不在 python 侧重造），
    并在报告中注明这是「端到端含签名与广播」的口径；
  - 输出 p50/p95/p99/max + RPS + 错误率，JSON 与文本双格式。

用法：
  python3 loadtest.py rpc   --url http://rpc:26657 --concurrency 1000 --duration 60
  python3 loadtest.py rest  --url https://api:1317 --qps 200 --duration 60
  python3 loadtest.py api   --url https://api.example/health --concurrency 200 --duration 600
  python3 loadtest.py tx    --home /node0 --to mc1abc... --count 500 --workers 20
"""
import argparse
import json
import os
import statistics
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
from collections import defaultdict

# ───────────────────────────── 公共 ─────────────────────────────

class Result:
    def __init__(self):
        self.lat = []          # 成功请求延迟 ms
        self.errors = defaultdict(int)
        self.total = 0
        self.lock = threading.Lock()

    def add_ok(self, ms):
        with self.lock:
            self.lat.append(ms)
            self.total += 1

    def add_err(self, kind):
        with self.lock:
            self.errors[kind] += 1
            self.total += 1

    def report(self, duration):
        lat = sorted(self.lat)
        n = len(lat)
        def pct(p):
            return lat[min(n - 1, int(n * p))] if n else 0
        rps = self.total / duration if duration > 0 else 0
        err = self.total - n
        out = {
            "total": self.total,
            "success": n,
            "errors": dict(self.errors),
            "error_rate": round(err / self.total, 4) if self.total else 0,
            "rps": round(rps, 2),
            "p50_ms": round(pct(0.50), 1),
            "p95_ms": round(pct(0.95), 1),
            "p99_ms": round(pct(0.99), 1),
            "max_ms": round(lat[-1], 1) if n else 0,
        }
        return out

    def pretty(self, tag, duration):
        r = self.report(duration)
        print(f"\n===== {tag} =====")
        for k, v in r.items():
            print(f"  {k:12s} {v}")
        verdict = "PASS" if r["error_rate"] < 0.01 and r["p99_ms"] < 800 else "CHECK"
        print(f"  verdict      {verdict}  (标准: err<1%, p99<800ms)")
        return r


def fetch(url, timeout=10, data=None, headers=None):
    """单次请求，返回 (ok, ms, err_kind)"""
    t0 = time.time()
    try:
        req = urllib.request.Request(url, data=data, headers=headers or {})
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            resp.read()
        return True, (time.time() - t0) * 1000, None
    except urllib.error.HTTPError as e:
        e.read()
        return False, (time.time() - t0) * 1000, f"http_{e.code}"
    except Exception as e:  # noqa: BLE001 — 压测工具要吞掉一切网络异常计为错误
        return False, (time.time() - t0) * 1000, type(e).__name__


def run_threads(worker_fn, nthreads, duration):
    """按 worker 线程跑满 duration 秒"""
    res = Result()
    stop = threading.Event()

    def worker():
        while not stop.is_set():
            ok, ms, err = worker_fn()
            if ok:
                res.add_ok(ms)
            else:
                res.add_err(err or "unknown")

    ts = [threading.Thread(target=worker, daemon=True) for _ in range(nthreads)]
    t0 = time.time()
    for t in ts:
        t.start()
    try:
        while time.time() - t0 < duration:
            time.sleep(0.5)
    except KeyboardInterrupt:
        print("\n[interrupt] 提前结束", file=sys.stderr)
    stop.set()
    for t in ts:
        t.join(timeout=5)
    return res, time.time() - t0


def run_qps(target_fn, qps, duration):
    """按恒定 QPS 节奏打（单线程调度 + 并发执行）"""
    res = Result()
    interval = 1.0 / qps
    t0 = time.time()
    sent = 0
    tail = []
    while True:
        now = time.time()
        elapsed = now - t0
        due = int(elapsed / interval)
        while sent < due:
            th = threading.Thread(target=lambda: (
                (lambda r: res.add_ok(r[1]) if r[0] else res.add_err(r[2]))(target_fn())
            ), daemon=True)
            th.start()
            tail.append(th)
            sent += 1
        if elapsed >= duration:
            break
        time.sleep(min(0.05, interval))
    # join 全部尾部请求：只 sleep 固定时长会让高 QPS 下的尾部请求结果丢失，
    # 统计虚低。join 带超时兜底，防线程悬挂。
    for th in tail[-2000:]:
        th.join(timeout=5)
    return res, time.time() - t0

# ───────────────────────────── 模式 ─────────────────────────────

def mode_rpc(args):
    """CometBFT JSON-RPC 查询：/status + /block（重查询）"""
    base = args.url.rstrip("/")
    def hit():
        if args.heavy and int(time.time()) % 2 == 0:
            return fetch(f"{base}/block?height=1")
        return fetch(f"{base}/status")
    res, dur = run_threads(hit, args.concurrency, args.duration)
    return res.pretty(f"RPC  查询（{args.concurrency} 并发 / {args.duration}s）", dur)


def mode_rest(args):
    """REST(LCD) 恒定 QPS"""
    base = args.url.rstrip("/")
    path = args.path or "/cosmos/base/tendermint/v1beta1/node_info"
    res, dur = run_qps(lambda: fetch(base + path), args.qps, args.duration)
    return res.pretty(f"REST 恒定 {args.qps} QPS / {args.duration}s", dur)


def mode_api(args):
    """APP 后端注册/业务 API 并发（注册潮模拟）"""
    url = args.url
    payload = json.dumps({"sim": 1, "ts": 0}).encode()
    headers = {"Content-Type": "application/json"}
    def hit():
        return fetch(url, data=payload, headers=headers)
    res, dur = run_threads(hit, args.concurrency, args.duration)
    return res.pretty(f"API  并发 {args.concurrency} / {args.duration}s", dur)


def mode_tx(args):
    """tx 广播端到端：mcchaind 并发子进程签名+广播。
    口径说明：含本地签名时间，测的是「端到端可广播速率」，非纯链端共识吞吐。
    验收：广播成功率 ≥ 99%，无 mempool 堵死。"""
    workers = args.workers
    errors = defaultdict(int)
    lat = []
    lock = threading.Lock()
    sent = [0]  # 按实际发送计数，不用 args.count 作分母

    def worker(i):
        # 不用 per = count // workers：整除失败时会丢余数笔（count 不能被
        # workers 整除），导致成功率虚高；改跨步分发，count 笔全发。
        for _ in range(i, args.count, workers):
            sent[0] += 1
            t0 = time.time()
            try:
                subprocess.run(
                    ["mcchaind", "tx", "bank", "send", "validator0", args.to,
                     "1umc", "--keyring-backend", "test", "--home", args.home,
                     "--chain-id", args.chain_id, "--node", args.node,
                     "--gas", "auto", "--gas-adjustment", "1.5",
                     "--fees", "5000umc", "-y"],
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                    check=True, timeout=30)
                with lock:
                    lat.append((time.time() - t0) * 1000)
            except Exception as e:  # noqa: BLE001
                with lock:
                    errors[type(e).__name__] += 1

    ts = [threading.Thread(target=worker, args=(i,)) for i in range(workers)]
    t0 = time.time()
    for t in ts:
        t.start()
    for t in ts:
        t.join()
    dur = time.time() - t0
    lat.sort()
    n = len(lat)
    total = sent[0]
    ok_rate = n / total if total else 0
    print(f"\n===== TX 广播端到端（{total} 笔 / {workers} workers）=====")
    print(f"  total        {total}")
    print(f"  success      {n}")
    print(f"  errors       {dict(errors)}")
    print(f"  success_rate {round(ok_rate, 4)}  (标准 ≥ 0.99)")
    print(f"  elapsed_s    {round(dur, 1)}")
    print(f"  tps          {round(total / dur, 2) if dur > 0 else 0}")
    if n:
        print(f"  p50_ms       {round(lat[n//2], 1)}")
        print(f"  p99_ms       {round(lat[min(n-1, int(n*0.99))], 1)}")
    verdict = "PASS" if ok_rate >= 0.99 else "CHECK"
    print(f"  verdict      {verdict}")
    return {"success_rate": ok_rate, "tps": round(total / dur, 2) if dur > 0 else 0}

# ───────────────────────────── CLI ─────────────────────────────

def main():
    p = argparse.ArgumentParser(description="MC 公链压测工具（纯标准库）")
    sub = p.add_subparsers(dest="mode", required=True)

    sp = sub.add_parser("rpc", help="CometBFT RPC 并发查询")
    sp.add_argument("--url", default="http://127.0.0.1:26657")
    sp.add_argument("--concurrency", type=int, default=1000)
    sp.add_argument("--duration", type=int, default=60)
    sp.add_argument("--heavy", action="store_true", help="混入 /block 重查询")

    sp = sub.add_parser("rest", help="REST 恒定 QPS")
    sp.add_argument("--url", default="http://127.0.0.1:1317")
    sp.add_argument("--qps", type=int, default=200)
    sp.add_argument("--duration", type=int, default=60)
    sp.add_argument("--path", default=None)

    sp = sub.add_parser("api", help="APP 后端 API 并发")
    sp.add_argument("--url", required=True)
    sp.add_argument("--concurrency", type=int, default=200)
    sp.add_argument("--duration", type=int, default=600)

    sp = sub.add_parser("tx", help="tx 广播端到端")
    sp.add_argument("--home", default="/var/lib/mcchain/.mcchaind")
    sp.add_argument("--to", required=True)
    sp.add_argument("--count", type=int, default=500)
    sp.add_argument("--workers", type=int, default=20)
    sp.add_argument("--chain-id", default="mcchain-mainnet-1")
    sp.add_argument("--node", default="http://127.0.0.1:26657")

    args = p.parse_args()
    {"rpc": mode_rpc, "rest": mode_rest, "api": mode_api, "tx": mode_tx}[args.mode](args)


if __name__ == "__main__":
    main()
