// Command indexer 是 MC 公链官方维护的事件索引器（docs/INDEXER.md）。
//
// 目标：
//   - **不抢节点资源**：独立进程，只读访问节点 RPC；
//   - **不丢块**：高水位 + 自动重连，断线重追；
//   - **可重启**：从 cursor 文件继续；
//   - **可插拔**：通过 EventSink 接口接入 JSONL / SQLite / Postgres 后端。
//
// 用法：
//
//	mcchain-indexer run   --node http://127.0.0.1:26657 --out /var/lib/mcchain/indexer
//	mcchain-indexer query --out /var/lib/mcchain/indexer --type transfer --limit 20
//
// 设计与性能数字：见 docs/INDEXER.md。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	ctrpclient "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

// ---------------------------------------------------------------------------
// 公共模型
// ---------------------------------------------------------------------------

// Block 是索引器持久化的最小块摘要。
type Block struct {
	Height   int64    `json:"height"`
	Hash     string   `json:"hash"`
	Time     int64    `json:"time"` // unix second
	Proposer string   `json:"proposer"`
	TxCount  int      `json:"tx_count"`
	TxHashes []string `json:"tx_hashes,omitempty"`
}

// Event 是单条链上事件。
type Event struct {
	Height int64             `json:"height"`
	TxHash string            `json:"tx_hash,omitempty"`
	Type   string            `json:"type"`
	Attrs  map[string]string `json:"attrs"`
}

// ---------------------------------------------------------------------------
// 后端抽象
// ---------------------------------------------------------------------------

// EventSink 把数据写到持久化层；它必须是线程安全的。
// 所有 error（除了 context.Canceled）都应被 indexer 当作可重试处理。
type EventSink interface {
	WriteEvents(ctx context.Context, evs []Event) error
	WriteBlock(ctx context.Context, b Block) error
	SetCursor(ctx context.Context, height int64) error
	Close() error
}

// ---------------------------------------------------------------------------
// JSONL 后端（默认）
// ---------------------------------------------------------------------------

// JSONLSink 用 append-only 文件落盘；blocks.jsonl / events.jsonl / cursor 单文件。
// 顺序：每块先追加 events，再追加 block 行；cursor 在两者之后写。
type JSONLSink struct {
	dir string

	mu          sync.Mutex
	blocksFile  *os.File
	eventsFile  *os.File
	cursorValue int64
}

// NewJSONLSink 打开（或创建）一个 JSONL 后端目录。
// 现存的 blocks.jsonl / events.jsonl 会被**追加**，所以重启不会丢数据。
func NewJSONLSink(dir string) (*JSONLSink, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create indexer dir: %w", err)
	}
	bf, err := os.OpenFile(filepath.Join(dir, "blocks.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open blocks.jsonl: %w", err)
	}
	ef, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		bf.Close()
		return nil, fmt.Errorf("open events.jsonl: %w", err)
	}
	c := &JSONLSink{dir: dir, blocksFile: bf, eventsFile: ef}
	// cursor 文件损坏/半写时 fail-fast —— 若静默
	// 回落 start-height 重新追加，导致重启后整段数据重复索引。
	if h, err := readCursorFile(filepath.Join(dir, "cursor")); err != nil {
		if !os.IsNotExist(err) {
			bf.Close()
			ef.Close()
			return nil, fmt.Errorf("read cursor (delete it only if you intend to re-index): %w", err)
		}
	} else {
		c.cursorValue = h
	}
	return c, nil
}

// Cursor 返回上次成功写入的最大高度。
func (s *JSONLSink) Cursor() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursorValue
}

func (s *JSONLSink) WriteEvents(ctx context.Context, evs []Event) error {
	if len(evs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range evs {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := s.eventsFile.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return s.eventsFile.Sync()
}

func (s *JSONLSink) WriteBlock(ctx context.Context, b Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	if _, err := s.blocksFile.Write(append(raw, '\n')); err != nil {
		return err
	}
	return s.blocksFile.Sync() // 保证 block 落盘后才前移 cursor
}

func (s *JSONLSink) SetCursor(ctx context.Context, height int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorValue = height
	// 原子写 —— 先写临时文件再 rename。原 os.WriteFile
	// 直接覆盖，进程在写入瞬间被 kill（systemd 强杀/断电）会留下空/半写文件，
	// 重启后触发上面的 fail-fast 或静默重复索引。
	tmp := filepath.Join(s.dir, "cursor.tmp")
	if err := os.WriteFile(tmp, []byte(strconv.FormatInt(height, 10)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "cursor"))
}

func (s *JSONLSink) Close() error {
	var errs []string
	if s.blocksFile != nil {
		if err := s.blocksFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if s.eventsFile != nil {
		if err := s.eventsFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func readCursorFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

// ---------------------------------------------------------------------------
// 索引主循环：拉块 → 写事件 → 写 block → 前移 cursor
// ---------------------------------------------------------------------------

// Indexer 把一条 mcchaind 的事件流落到一个 EventSink。
type Indexer struct {
	node   string
	sink   EventSink
	fromH  int64 // 启动高度；若 sink 已有 cursor 则自动覆盖
	period time.Duration
}

// NewIndexer 构造一个 indexer。node 是 CometBFT RPC HTTP 地址。
func NewIndexer(node string, sink EventSink, fromHeight int64) *Indexer {
	return &Indexer{
		node:   node,
		sink:   sink,
		fromH:  fromHeight,
		period: 2 * time.Second,
	}
}

// Run 阻塞直到 ctx 取消；内部按 cursor 推进。
func (ix *Indexer) Run(ctx context.Context) error {
	// 如果 sink 已经有 cursor，从 cursor + 1 继续；否则用 --start-height。
	if cur := cursorOf(ix.sink); cur > 0 {
		ix.fromH = cur + 1
	}

	c, err := ctrpclient.New(ix.node, "/websocket")
	if err != nil {
		return fmt.Errorf("connect node: %w", err)
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("start rpc client: %w", err)
	}
	defer c.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := c.Status(ctx)
		if err != nil {
			// 失败必须记日志。静默重试时，
			// 剪枝节点 cursor 落后于 earliest_block_height 时会 1s 一次
			// 无限空转且无任何可观测信号。
			log.Printf("[indexer] status fetch failed (retrying): %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		latest := status.SyncInfo.LatestBlockHeight

		for ix.fromH <= latest {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err := ix.indexOne(ctx, c, ix.fromH); err != nil {
				// 写盘失败时不前移 cursor；下次重试同一高度
				log.Printf("[indexer] index height %d failed (retrying): %v", ix.fromH, err)
				time.Sleep(time.Second)
				continue
			}
			ix.fromH++
		}
		time.Sleep(ix.period)
	}
}

func (ix *Indexer) indexOne(ctx context.Context, c *ctrpclient.HTTP, height int64) error {
	b, err := c.Block(ctx, &height)
	if err != nil {
		return fmt.Errorf("fetch block %d: %w", height, err)
	}

	events, err := extractEvents(ctx, c, height)
	if err != nil {
		return err
	}
	if err := ix.sink.WriteEvents(ctx, events); err != nil {
		return fmt.Errorf("write events: %w", err)
	}

	txs := []string{}
	if b.Block != nil {
		for _, t := range b.Block.Txs {
			txs = append(txs, fmt.Sprintf("%X", t.Hash()))
		}
	}

	block := Block{
		Height:   height,
		Hash:     b.BlockID.Hash.String(),
		Time:     b.Block.Header.Time.Unix(),
		Proposer: b.Block.Header.ProposerAddress.String(),
		TxCount:  len(b.Block.Txs),
		TxHashes: txs,
	}
	if err := ix.sink.WriteBlock(ctx, block); err != nil {
		return fmt.Errorf("write block: %w", err)
	}

	if err := ix.sink.SetCursor(ctx, height); err != nil {
		return fmt.Errorf("set cursor: %w", err)
	}
	return nil
}

// extractEvents 抓 BeginBlock / EndBlock / 每笔 tx 的事件。
func extractEvents(ctx context.Context, c *ctrpclient.HTTP, height int64) ([]Event, error) {
	res, err := c.BlockResults(ctx, &height)
	if err != nil {
		return nil, fmt.Errorf("fetch block results %d: %w", height, err)
	}
	var out []Event
	for _, ev := range res.BeginBlockEvents {
		out = append(out, eventFrom(height, "", ev))
	}
	for _, ev := range res.EndBlockEvents {
		out = append(out, eventFrom(height, "", ev))
	}
	// tx 级别事件：events 是按 txs 顺序对齐的 slice of slice
	for i, t := range res.TxsResults {
		if t == nil {
			continue
		}
		txHash := fmt.Sprintf("tx-%d-%d", height, i)
		for _, ev := range t.Events {
			out = append(out, eventFrom(height, txHash, ev))
		}
	}
	return out, nil
}

func eventFrom(height int64, txHash string, ev abci.Event) Event {
	attrs := make(map[string]string, len(ev.Attributes))
	for _, a := range ev.Attributes {
		attrs[a.Key] = a.Value
	}
	return Event{Height: height, TxHash: txHash, Type: ev.Type, Attrs: attrs}
}

// cursorOf 兼容 sink.Cursor 的方法（如果存在）。
func cursorOf(s EventSink) int64 {
	type cur interface{ Cursor() int64 }
	if c, ok := s.(cur); ok {
		return c.Cursor()
	}
	return 0
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "query":
		queryCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `mcchain-indexer — chain event indexer (docs/INDEXER.md)

Commands:
  run   [flags]    index new blocks continuously
  query [flags]    read from a local index

run flags:
  --node         URL of CometBFT RPC (default http://127.0.0.1:26657)
  --out          indexer output dir (default ./indexer-data)
  --start-height height to start from if cursor is missing (default 1)

query flags:
  --out          indexer dir (default ./indexer-data)
  --type         event type filter (substring, e.g. 'transfer')
  --limit        max events to print (default 20)`)
}

type runFlags struct {
	node        string
	out         string
	startHeight int64
}

func parseRunFlags(args []string) runFlags {
	f := runFlags{
		node:        "http://127.0.0.1:26657",
		out:         "indexer-data",
		startHeight: 1,
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--node":
			i++
			f.node = args[i]
		case "--out":
			i++
			f.out = args[i]
		case "--start-height":
			i++
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err == nil {
				f.startHeight = n
			}
		}
	}
	return f
}

func runCmd(args []string) {
	f := parseRunFlags(args)
	sink, err := NewJSONLSink(f.out)
	if err != nil {
		die(err)
	}
	defer sink.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ix := NewIndexer(f.node, sink, f.startHeight)
	fmt.Printf("[indexer] start, node=%s out=%s\n", f.node, f.out)
	if err := ix.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		die(err)
	}
}

type queryFlags struct {
	out   string
	typ   string
	limit int
}

func parseQueryFlags(args []string) queryFlags {
	f := queryFlags{out: "indexer-data", limit: 20}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			f.out = args[i]
		case "--type":
			i++
			f.typ = args[i]
		case "--limit":
			i++
			n, _ := strconv.Atoi(args[i])
			f.limit = n
		}
	}
	return f
}

func queryCmd(args []string) {
	f := parseQueryFlags(args)
	path := filepath.Join(f.out, "events.jsonl")
	file, err := os.Open(path)
	if err != nil {
		die(fmt.Errorf("open %s: %w", path, err))
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	count := 0
	for {
		var e Event
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			die(err)
		}
		if f.typ != "" && !strings.Contains(e.Type, f.typ) {
			continue
		}
		raw, _ := json.Marshal(e)
		fmt.Println(string(raw))
		count++
		if count >= f.limit {
			break
		}
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "no matching events")
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "fatal:", err)
	os.Exit(1)
}

// 防「未使用」告警：ResultBlockResults 的字段名只在签名里出现过。
var _ coretypes.ResultBlockResults
