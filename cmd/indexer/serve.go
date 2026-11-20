package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// serveCmd 启动只读 HTTP 查询服务。
//
// 设计约束：
//   - 只读：仅暴露 GET，不写索引、不触发重扫；
//   - 单进程：与 run 共用同一个输出目录时只读取已落盘 JSONL；
//   - 有界内存：每个请求只保留最近 limit+offset 条匹配记录，避免大文件撑爆内存；
//   - 无第三方依赖：仅标准库。
func serveCmd(args []string) {
	f := parseServeFlags(args)
	server := &http.Server{
		Addr:              f.addr,
		Handler:           NewReadOnlyHandler(f.out),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("[indexer] serve, addr=%s out=%s\n", f.addr, f.out)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		die(err)
	}
}

type serveFlags struct {
	out  string
	addr string
}

func parseServeFlags(args []string) serveFlags {
	f := serveFlags{out: "indexer-data", addr: "127.0.0.1:8090"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i < len(args) {
				f.out = args[i]
			}
		case "--addr":
			i++
			if i < len(args) {
				f.addr = args[i]
			}
		}
	}
	return f
}

// ReadOnlyHandler 提供索引数据的只读查询。
type ReadOnlyHandler struct {
	dir string
}

func NewReadOnlyHandler(dir string) http.Handler {
	h := &ReadOnlyHandler{dir: dir}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/v1/events", h.events)
	mux.HandleFunc("/v1/contributions", h.contributions)
	mux.HandleFunc("/v1/blocks", h.blocks)
	return mux
}

func (h *ReadOnlyHandler) health(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	cursor := int64(0)
	if v, err := readCursorFile(filepath.Join(h.dir, "cursor")); err == nil {
		cursor = v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"cursor": cursor,
	})
}

func (h *ReadOnlyHandler) events(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	limit, offset := pagination(r)
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	address := strings.TrimSpace(r.URL.Query().Get("address"))

	items, total, err := h.loadEvents(limit, offset, func(e Event) bool {
		if typeFilter != "" && !strings.Contains(e.Type, typeFilter) {
			return false
		}
		return address == "" || attrsContain(e.Attrs, address)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *ReadOnlyHandler) contributions(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	limit, offset := pagination(r)
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		writeError(w, http.StatusBadRequest, errors.New("address is required"))
		return
	}

	items, total, err := h.loadEvents(limit, offset, func(e Event) bool {
		if e.Type != "depin.RewardPaid" && e.Type != "depin.DefenseBlocked" {
			return false
		}
		return e.Attrs["device"] == address
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":  items,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"address": address,
	})
}

func (h *ReadOnlyHandler) blocks(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	limit, offset := pagination(r)
	file, err := os.Open(filepath.Join(h.dir, "blocks.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"blocks": []Block{}, "total": 0, "limit": limit, "offset": offset})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	window := make([]Block, 0, limit)
	total := 0
	for {
		var b Block
		if err := dec.Decode(&b); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		total++
		window = append(window, b)
		if len(window) > limit+offset {
			window = window[1:]
		}
	}
	reverseBlocks(window)
	page := window
	if offset < len(window) {
		end := offset + limit
		if end > len(window) {
			end = len(window)
		}
		page = window[offset:end]
	} else {
		page = []Block{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"blocks": page,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// loadEvents 扫描 events.jsonl，返回按高度倒序的指定分页。
// 只保留最近 limit+offset 条匹配项，内存占用与文件大小无关。
func (h *ReadOnlyHandler) loadEvents(limit, offset int, keep func(Event) bool) ([]Event, int, error) {
	file, err := os.Open(filepath.Join(h.dir, "events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, 0, nil
		}
		return nil, 0, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	window := make([]Event, 0, limit)
	total := 0
	for {
		var e Event
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, err
		}
		if keep != nil && !keep(e) {
			continue
		}
		total++
		window = append(window, e)
		if len(window) > limit+offset {
			window = window[1:]
		}
	}
	reverseEvents(window)
	page := window
	if offset < len(window) {
		end := offset + limit
		if end > len(window) {
			end = len(window)
		}
		page = window[offset:end]
	} else {
		page = []Event{}
	}
	return page, total, nil
}

func pagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 200 {
		limit = 200
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return limit, offset
}

func attrsContain(attrs map[string]string, needle string) bool {
	for _, value := range attrs {
		if value == needle {
			return true
		}
	}
	return false
}

func reverseEvents(items []Event) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func reverseBlocks(items []Block) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func allowGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeError(w, http.StatusMethodNotAllowed, errors.New("only GET is allowed"))
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
