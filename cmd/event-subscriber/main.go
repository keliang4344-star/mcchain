package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client/http"
	cmttypes "github.com/cometbft/cometbft/types"
)

// 链下最小验证工具：事件订阅 + 指标导出/持久化。
// 连接任意 mcchaind Tendermint RPC 端点，订阅 depin / phonenode / edgeai 模块的
// 关键业务事件（挖矿到账、节点 slash、节点认证、EdgeAI 争议与拨付）并打印，
// 同时：
//   - 按事件类型聚合计数，并通过 Prometheus 暴露 /metrics（默认 :2112）；
//   - 周期性 / 退出时将计数摘要持久化为 JSON 文件（默认 event_metrics.json）。
//
// 用法：go run ./cmd/event-subscriber [rpc-url]
// 默认 rpc-url = "http://localhost:26657"
// 环境变量：
//   MC_SUB_METRICS_ADDR  Prometheus 监听地址（默认 ":2112"，留空则关闭）
//   MC_SUB_METRICS_FILE  JSON 摘要输出路径（默认 "event_metrics.json"，留空则关闭）

// mcEventTypes 需关注的事件类型（SDK 事件契约）。
var mcEventTypes = map[string]bool{
	"depin.RewardPaid":       true,
	"phonenode.Slash":        true,
	"phonenode.Attestation":  true,
	"edgeai.TaskCreated":     true,
	"edgeai.ResultSubmitted": true,
	"edgeai.DisputeOpened":   true,
	"edgeai.RewardPaid":      true,
	"edgeai.DisputeResolved": true,
}

// eventCounter Prometheus 计数器（按事件类型打标签）。
var eventCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "mc",
		Subsystem: "subscriber",
		Name:      "mc_events_total",
		Help:      "Total number of MC business events observed by the event-subscriber, by event type.",
	},
	[]string{"event"},
)

// summary 维护内存中的聚合计数，供 JSON 持久化使用。
type summary struct {
	mu        sync.Mutex
	counts    map[string]int64
	total     int64
	lastSeen  map[string]time.Time
	startedAt time.Time
}

func newSummary() *summary {
	return &summary{
		counts:    make(map[string]int64),
		lastSeen:  make(map[string]time.Time),
		startedAt: time.Now(),
	}
}

func (s *summary) observe(eventType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[eventType]++
	s.total++
	s.lastSeen[eventType] = time.Now()
}

func (s *summary) snapshot() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	lastSeen := make(map[string]string, len(s.lastSeen))
	for k, v := range s.lastSeen {
		lastSeen[k] = v.Format(time.RFC3339)
	}
	return map[string]interface{}{
		"started_at":   s.startedAt.Format(time.RFC3339),
		"snapshot_at":  time.Now().Format(time.RFC3339),
		"total_events": s.total,
		"counts":       s.counts,
		"last_seen":    lastSeen,
	}
}

// totalEvents 竞态安全地读取总事件数（SEC：直读 s.total 与 observe 数据竞争）。
func (s *summary) totalEvents() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.total
}

func main() {
	rpcURL := "http://localhost:26657"
	if len(os.Args) > 1 {
		rpcURL = os.Args[1]
	}

	metricsAddr := os.Getenv("MC_SUB_METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}
	metricsFile := os.Getenv("MC_SUB_METRICS_FILE")
	if metricsFile == "" {
		metricsFile = "event_metrics.json"
	}

	if err := prometheus.Register(eventCounter); err != nil {
		// 重复注册（如测试）时忽略。
		log.Printf("[event-subscriber] prometheus register skipped: %v", err)
	}

	sum := newSummary()

	// 启动 Prometheus /metrics 端点。
	if metricsAddr != "off" && metricsAddr != "disabled" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			log.Printf("[event-subscriber] metrics server listening on %s/metrics", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, mux); err != nil {
				log.Printf("[event-subscriber] metrics server stopped: %v", err)
			}
		}()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 订阅全部 Tx，链下按事件类型过滤（cometbft 多类型 OR 订阅受限，链下过滤更稳）。
	query := "tm.event='Tx'"
	fmt.Printf("[event-subscriber] subscribing: %s ; watching MC events: %s\n", query, strings.Join(keys(mcEventTypes), ", "))

	// 断线重连。Subscribe 后若不重连：
	// 节点重启/网络抖动时 CometBFT 客户端关闭订阅 channel，`<-ch` 永远返回
	// 零值 nil，主循环按 unknown event 100% CPU 空转且指标归零无告警。
	// 现改为：channel 关闭 → 退避后重新连接并重新订阅；无法恢复才退出。
	//
	// 竞态修复：sum.total 直接读取绕过了 mutex（与 observe 数据竞争，
	// -race 可复现），改走 snapshot()；ctx.Done 时由主循环自己 persist 再返回
	// （原 goroutine 与主循环 return 竞争，最后一次快照可能没写完）。
	persist := func() {
		if metricsFile == "off" || metricsFile == "disabled" {
			return
		}
		bz, err := json.MarshalIndent(sum.snapshot(), "", "  ")
		if err != nil {
			log.Printf("[event-subscriber] marshal metrics summary failed: %v", err)
			return
		}
		if err := os.WriteFile(metricsFile, bz, 0o644); err != nil {
			log.Printf("[event-subscriber] write metrics file %s failed: %v", metricsFile, err)
		} else {
			log.Printf("[event-subscriber] metrics summary persisted to %s (%d events)", metricsFile, sum.totalEvents())
		}
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	eventCount := int64(0)
	backoff := 5 * time.Second
	for {
		subCtx, subCancel := context.WithCancel(ctx)
		client, err := rpcclient.New(rpcURL, "/websocket")
		if err != nil {
			subCancel()
			log.Printf("[event-subscriber] connect to %s failed: %v (retry in %s)", rpcURL, err, backoff)
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				persist()
				return
			}
		}
		ch, err := client.Subscribe(subCtx, "mcchain-b5-sub", query)
		if err != nil {
			_ = client.Stop()
			subCancel()
			log.Printf("[event-subscriber] subscribe failed: %v (retry in %s)", err, backoff)
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				persist()
				return
			}
		}
		fmt.Printf("[event-subscriber] connected to %s\n", rpcURL)
		backoff = 5 * time.Second // 连接成功后重置退避

	waitLoop:
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					// channel 被服务端关闭（节点重启/网络断开）→ 断线重连。
					log.Printf("[event-subscriber] subscription channel closed, reconnecting in %s", backoff)
					_ = client.Stop()
					subCancel()
					break waitLoop
				}
				handle(ev, sum, eventCounter)
				eventCount++
				// 每 1000 个事件落盘一次，降低崩溃丢数风险。
				if eventCount%1000 == 0 {
					persist()
				}
			case <-ticker.C:
				persist()
			case <-ctx.Done():
				persist()
				_ = client.Stop()
				subCancel()
				return
			}
		}

		select {
		case <-time.After(backoff):
			if backoff < time.Minute {
				backoff *= 2 // 指数退避，封顶 1 分钟
			}
		case <-ctx.Done():
			persist()
			return
		}
	}
}

func handle(ev interface{}, sum *summary, counter *prometheus.CounterVec) {
	txData, ok := ev.(cmttypes.EventDataTx)
	if !ok {
		fmt.Printf("[event-subscriber] (unknown event type) %T\n", ev)
		return
	}
	for _, e := range txData.Result.Events {
		if !mcEventTypes[e.Type] {
			continue
		}
		printEvent(e)
		sum.observe(e.Type)
		counter.WithLabelValues(e.Type).Inc()
	}
}

func printEvent(e abci.Event) {
	attrs := make([]string, 0, len(e.Attributes))
	for _, a := range e.Attributes {
		attrs = append(attrs, fmt.Sprintf("%s=%s", string(a.Key), string(a.Value)))
	}
	fmt.Printf("[event-subscriber] %s | %s\n", e.Type, strings.Join(attrs, " "))
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
