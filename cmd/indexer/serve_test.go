package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTestIndex(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	events := []Event{
		{Height: 1, Type: "depin.RewardPaid", Attrs: map[string]string{"device": "mc1A", "task_id": "t1", "payout": "1000"}},
		{Height: 2, Type: "depin.RewardPaid", Attrs: map[string]string{"device": "mc1B", "task_id": "t2", "payout": "2000"}},
		{Height: 3, Type: "depin.DefenseBlocked", Attrs: map[string]string{"device": "mc1A", "failed_layer": "3"}},
		{Height: 4, Type: "transfer", Attrs: map[string]string{"recipient": "mc1A", "amount": "5"}},
		{Height: 5, Type: "depin.RewardPaid", Attrs: map[string]string{"device": "mc1A", "task_id": "t3", "payout": "3000"}},
	}
	var buf []byte
	for _, e := range events {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		buf = append(append(buf, raw...), '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	blocks := []Block{
		{Height: 1, Hash: "h1", TxCount: 0},
		{Height: 2, Hash: "h2", TxCount: 1},
	}
	buf = nil
	for _, b := range blocks {
		raw, _ := json.Marshal(b)
		buf = append(append(buf, raw...), '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "blocks.jsonl"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cursor"), []byte("42"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHealthReportsCursor(t *testing.T) {
	dir := writeTestIndex(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	NewReadOnlyHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Cursor int64  `json:"cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Cursor != 42 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestContributionsFilterAndPaging(t *testing.T) {
	dir := writeTestIndex(t)
	handler := NewReadOnlyHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/v1/contributions?address=mc1A&limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []Event `json:"events"`
		Total  int     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 3 {
		t.Fatalf("total=%d, want 3", body.Total)
	}
	if len(body.Events) != 2 || body.Events[0].Height != 5 || body.Events[1].Height != 3 {
		t.Fatalf("unexpected page: %+v", body.Events)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/contributions?address=mc1A&limit=2&offset=2", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 1 || body.Events[0].Height != 1 {
		t.Fatalf("unexpected second page: %+v", body.Events)
	}
}

func TestEventsAddressFilterMatchesAnyAttribute(t *testing.T) {
	dir := writeTestIndex(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/events?address=mc1A&limit=10", nil)
	rec := httptest.NewRecorder()
	NewReadOnlyHandler(dir).ServeHTTP(rec, req)

	var body struct {
		Events []Event `json:"events"`
		Total  int     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 4 {
		t.Fatalf("total=%d, want 4", body.Total)
	}
	if len(body.Events) != 4 || body.Events[0].Height != 5 || body.Events[3].Height != 1 {
		t.Fatalf("unexpected order: %+v", body.Events)
	}
}

func TestBlocksPaging(t *testing.T) {
	dir := writeTestIndex(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/blocks?limit=1", nil)
	rec := httptest.NewRecorder()
	NewReadOnlyHandler(dir).ServeHTTP(rec, req)

	var body struct {
		Blocks []Block `json:"blocks"`
		Total  int     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 2 || len(body.Blocks) != 1 || body.Blocks[0].Height != 2 {
		t.Fatalf("unexpected blocks: %+v total=%d", body.Blocks, body.Total)
	}
}

func TestOnlyGetAllowed(t *testing.T) {
	dir := writeTestIndex(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	rec := httptest.NewRecorder()
	NewReadOnlyHandler(dir).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestMissingIndexReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	rec := httptest.NewRecorder()
	NewReadOnlyHandler(dir).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []Event `json:"events"`
		Total  int     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 0 || len(body.Events) != 0 {
		t.Fatalf("expected empty, got %+v", body)
	}
}
