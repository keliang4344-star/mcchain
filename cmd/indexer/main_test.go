package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// readCursorFile
// ---------------------------------------------------------------------------

func TestReadCursorFileOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cursor")
	if err := os.WriteFile(p, []byte("  12345  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := readCursorFile(p)
	if err != nil {
		t.Fatalf("readCursorFile: %v", err)
	}
	if h != 12345 {
		t.Fatalf("got %d, want 12345", h)
	}
}

func TestReadCursorFileNotExist(t *testing.T) {
	if _, err := readCursorFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestReadCursorFileCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cursor")
	if err := os.WriteFile(p, []byte("not-a-number"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCursorFile(p); err == nil {
		t.Fatal("want error for corrupt cursor")
	}
}

// ---------------------------------------------------------------------------
// JSONLSink
// ---------------------------------------------------------------------------

func TestJSONLSinkRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONLSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := s.WriteEvents(ctx, []Event{
		{Height: 1, Type: "transfer", Attrs: map[string]string{"to": "mc1x", "amount": "100"}},
		{Height: 1, Type: "mint", Attrs: map[string]string{"amount": "500"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteBlock(ctx, Block{Height: 1, Hash: "AAAA", Time: 1700000000, Proposer: "mcval1", TxCount: 2, TxHashes: []string{"T1", "T2"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCursor(ctx, 1); err != nil {
		t.Fatal(err)
	}

	// cursor 值
	if got := s.Cursor(); got != 1 {
		t.Fatalf("cursor = %d, want 1", got)
	}

	// blocks.jsonl 追加且带换行
	bb, err := os.ReadFile(filepath.Join(dir, "blocks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(bb)), "\n")
	if len(lines) != 1 {
		t.Fatalf("blocks lines = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], `"height":1`) || !strings.Contains(lines[0], `"tx_hashes"`) {
		t.Fatalf("block line unexpected: %s", lines[0])
	}

	// events.jsonl 两行
	eb, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(strings.TrimSpace(string(eb)), "\n")); n != 2 {
		t.Fatalf("events lines = %d, want 2", n)
	}

	// cursor 原子写：无 tmp 残留
	if _, err := os.Stat(filepath.Join(dir, "cursor.tmp")); !os.IsNotExist(err) {
		t.Fatal("cursor.tmp should not remain after SetCursor")
	}
	cb, err := os.ReadFile(filepath.Join(dir, "cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cb) != "1" {
		t.Fatalf("cursor file = %q, want \"1\"", string(cb))
	}
}

// TestJSONLSinkReopenAppends：重启（重新 NewJSONLSink）后旧数据仍在，新写入是追加而非覆盖。
func TestJSONLSinkReopenAppends(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewJSONLSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s1.WriteEvents(ctx, []Event{{Height: 1, Type: "transfer"}}); err != nil {
		t.Fatal(err)
	}
	if err := s1.SetCursor(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	// 模拟重启：从 cursor 继续
	s2, err := NewJSONLSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if got := s2.Cursor(); got != 1 {
		t.Fatalf("reopened cursor = %d, want 1", got)
	}
	if err := s2.WriteEvents(ctx, []Event{{Height: 2, Type: "transfer"}}); err != nil {
		t.Fatal(err)
	}
	eb, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(strings.TrimSpace(string(eb)), "\n")); n != 2 {
		t.Fatalf("after reopen events lines = %d, want 2 (append not overwrite)", n)
	}
}

// TestJSONLSinkCorruptCursorFailFast：cursor 损坏时必须拒绝启动，避免静默重复索引。
func TestJSONLSinkCorruptCursorFailFast(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cursor"), []byte("oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONLSink(dir); err == nil {
		t.Fatal("want error when cursor file is corrupt (fail-fast)")
	}
}

// TestJSONLSinkEmptyEventsNoOp：空事件列表不应触碰文件。
func TestJSONLSinkEmptyEventsNoOp(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONLSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.WriteEvents(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != 0 {
		t.Fatalf("events.jsonl size = %d, want 0 for empty batch", st.Size())
	}
}

// TestSetCursorOverwrite：SetCursor 显式覆盖内存与文件，且每次原子写后无 tmp 残留。
// 索引器主循环只单向前移，但 SetCursor 本身是「显式设置」语义，允许覆盖。
func TestSetCursorOverwrite(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSONLSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.SetCursor(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCursor(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if got := s.Cursor(); got != 7 {
		t.Fatalf("cursor = %d, want 7 (last write wins)", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "cursor.tmp")); !os.IsNotExist(err) {
		t.Fatal("cursor.tmp should not remain after SetCursor")
	}
	cb, err := os.ReadFile(filepath.Join(dir, "cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cb) != "7" {
		t.Fatalf("cursor file = %q, want \"7\"", string(cb))
	}
}
