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
	cmttypes "github.com/cometbft/cometbft/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client/http"
)

// B5 链下最小验证工具：事件订阅 + 指标导出/持久化。
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

// mcEventTypes 需关注的事件类型（B5 SDK 事件契约）。
var mcEventTypes = map[string]bool{
	"depin.RewardPaid":      true,
	"phonenode.Slash":       true,
	"phonenode.Attestation": true,
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
		"started_at": s.startedAt.Format(time.RFC3339),
		"snapshot_at": time.Now().Format(time.RFC3339),
		"total_events": s.total,
		"counts":       s.counts,
		"last_seen":    lastSeen,
	}
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
		log.Printf("[B5] prometheus register skipped: %v", err)
	}

	sum := newSummary()

	// 启动 Prometheus /metrics 端点。
	if metricsAddr != "off" && metricsAddr != "disabled" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			log.Printf("[B5] metrics server listening on %s/metrics", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, mux); err != nil {
				log.Printf("[B5] metrics server stopped: %v", err)
			}
		}()
	}

	client, err := rpcclient.New(rpcURL, "/websocket")
	if err != nil {
		log.Fatalf("connect to %s: %v", rpcURL, err)
	}
	defer client.Stop()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 订阅全部 Tx，链下按事件类型过滤（cometbft 多类型 OR 订阅受限，链下过滤更稳）。
	query := "tm.event='Tx'"
	fmt.Printf("[B5 event-subscriber] connected to %s\n", rpcURL)
	fmt.Printf("[B5] subscribing: %s ; watching MC events: %s\n", query, strings.Join(keys(mcEventTypes), ", "))

	ch, err := client.Subscribe(ctx, "mcchain-b5-sub", query)
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	fmt.Println("[B5] listening...")

	persist := func() {
		if metricsFile == "off" || metricsFile == "disabled" {
			return
		}
		bz, err := json.MarshalIndent(sum.snapshot(), "", "  ")
		if err != nil {
			log.Printf("[B5] marshal metrics summary failed: %v", err)
			return
		}
		if err := os.WriteFile(metricsFile, bz, 0o644); err != nil {
			log.Printf("[B5] write metrics file %s failed: %v", metricsFile, err)
		} else {
			log.Printf("[B5] metrics summary persisted to %s (%d events)", metricsFile, sum.total)
		}
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		<-ctx.Done()
		fmt.Println("[B5] shutting down")
		persist()
	}()

	eventCount := int64(0)
	for {
		select {
		case ev := <-ch:
			handle(ev, sum, eventCounter)
			eventCount++
			// 每 1000 个事件落盘一次，降低崩溃丢数风险。
			if eventCount%1000 == 0 {
				persist()
			}
		case <-ticker.C:
			persist()
		case <-ctx.Done():
			return
		}
	}
}

func handle(ev interface{}, sum *summary, counter *prometheus.CounterVec) {
	txData, ok := ev.(cmttypes.EventDataTx)
	if !ok {
		fmt.Printf("[B5] (unknown event type) %T\n", ev)
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
	fmt.Printf("[B5] %s | %s\n", e.Type, strings.Join(attrs, " "))
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
