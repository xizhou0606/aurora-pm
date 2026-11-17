package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	releasenotesgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const testInternalSecret = "test-internal-secret-at-least-32-characters"

func newReleaseNotesTestApp(t *testing.T, secret string) *testApp {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AI.InternalSecret = secret
	})
	if err := app.db.AutoMigrate(&releasenotesgorm.ReleaseNote{}); err != nil {
		t.Fatal(err)
	}
	return app
}

func signInternalBody(secret string, raw []byte, at time.Time) (string, string) {
	timestamp := strconv.FormatInt(at.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(raw)
	return timestamp, hex.EncodeToString(mac.Sum(nil))
}

func postReleaseNotes(app *testApp, raw []byte, secret string, at time.Time) *httptest.ResponseRecorder {
	timestamp, signature := signInternalBody(secret, raw, at)
	req := httptest.NewRequest(http.MethodPost, "/internal/release-notes", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PM-Timestamp", timestamp)
	req.Header.Set("X-PM-Signature", signature)
	rec := httptest.NewRecorder()
	app.engine.ServeHTTP(rec, req)
	return rec
}

func TestCreateReleaseNoteRejectsInvalidSignature(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	raw := []byte(`{"version":"1.0.0","contentMd":"# notes"}`)
	wrongSecretTimestamp, wrongSecretSignature := signInternalBody("another-internal-secret-with-32-chars!", raw, time.Now())
	cases := []struct {
		name      string
		timestamp string
		signature string
	}{
		{name: "wrong secret", timestamp: wrongSecretTimestamp, signature: wrongSecretSignature},
		{name: "missing headers", timestamp: "", signature: ""},
		{name: "stale timestamp", timestamp: strconv.FormatInt(
			time.Now().Add(-internalSignatureMaxSkew-time.Minute).UnixMilli(), 10), signature: "ff"},
		{name: "non-numeric timestamp", timestamp: "not-a-number", signature: "ff"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/internal/release-notes", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-PM-Timestamp", testCase.timestamp)
			req.Header.Set("X-PM-Signature", testCase.signature)
			rec := httptest.NewRecorder()
			app.engine.ServeHTTP(rec, req)
			requireStatus(t, rec, http.StatusUnauthorized)
		})
	}
}

func TestCreateReleaseNoteFailsClosedWithoutSecret(t *testing.T) {
	app := newReleaseNotesTestApp(t, "")
	raw := []byte(`{"version":"1.0.0","contentMd":"# notes"}`)
	timestamp, signature := signInternalBody(testInternalSecret, raw, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/internal/release-notes", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PM-Timestamp", timestamp)
	req.Header.Set("X-PM-Signature", signature)
	rec := httptest.NewRecorder()
	app.engine.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestCreateReleaseNoteIdempotentAndConflict(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	body := func(sha string) []byte {
		return []byte(fmt.Sprintf(`{"version":"1.0.0","releaseSha":%q,"contentMd":"# notes","source":"deploy"}`, sha))
	}
	first := postReleaseNotes(app, body("abc123"), testInternalSecret, time.Now())
	requireStatus(t, first, http.StatusOK)
	replay := postReleaseNotes(app, body("abc123"), testInternalSecret, time.Now())
	requireStatus(t, replay, http.StatusOK)
	if replay.Body.String() != first.Body.String() {
		t.Fatalf("idempotent replay must return the stored row: first=%s replay=%s",
			first.Body.String(), replay.Body.String())
	}
	conflict := postReleaseNotes(app, body("def456"), testInternalSecret, time.Now())
	requireStatus(t, conflict, http.StatusConflict)
	var count int64
	if err := app.db.Table("release_notes").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count=%d, want 1", count)
	}
}

func TestCreateReleaseNoteValidatesPayload(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	cases := map[string][]byte{
		"invalid json":     []byte(`{version:`),
		"missing version":  []byte(`{"contentMd":"# notes"}`),
		"missing content":  []byte(`{"version":"1.0.0"}`),
		"blank content md": []byte(`{"version":"1.0.0","contentMd":"   "}`),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postReleaseNotes(app, raw, testInternalSecret, time.Now())
			requireStatus(t, rec, http.StatusBadRequest)
		})
	}
}

func getLatestReleaseNote(app *testApp, secret string, at time.Time) *httptest.ResponseRecorder {
	timestamp, signature := signInternalBody(secret, nil, at)
	req := httptest.NewRequest(http.MethodGet, "/internal/release-notes/latest", nil)
	req.Header.Set("X-PM-Timestamp", timestamp)
	req.Header.Set("X-PM-Signature", signature)
	rec := httptest.NewRecorder()
	app.engine.ServeHTTP(rec, req)
	return rec
}

func TestLatestReleaseNoteRejectsInvalidSignature(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	wrongTimestamp, wrongSignature := signInternalBody("another-internal-secret-with-32-chars!", nil, time.Now())
	cases := []struct {
		name      string
		timestamp string
		signature string
	}{
		{name: "wrong secret", timestamp: wrongTimestamp, signature: wrongSignature},
		{name: "missing headers", timestamp: "", signature: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/internal/release-notes/latest", nil)
			req.Header.Set("X-PM-Timestamp", testCase.timestamp)
			req.Header.Set("X-PM-Signature", testCase.signature)
			rec := httptest.NewRecorder()
			app.engine.ServeHTTP(rec, req)
			requireStatus(t, rec, http.StatusUnauthorized)
		})
	}
}

func TestLatestReleaseNoteReturnsNewestNoteOrEmpty(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	if rec := getLatestReleaseNote(app, testInternalSecret, time.Now()); rec.Code != http.StatusOK {
		t.Fatalf("empty store: status=%d body=%s", rec.Code, rec.Body.String())
	} else if data := decodeData(t, rec); data != nil {
		t.Fatalf("empty store must return null data, got %v", data)
	}
	now := time.Now()
	for _, seed := range []struct{ version, sha string }{
		{"1.0.0", "aaa"}, {"1.1.0", "bbb"},
	} {
		raw := []byte(fmt.Sprintf(`{"version":%q,"releaseSha":%q,"contentMd":"# %s"}`, seed.version, seed.sha, seed.version))
		if rec := postReleaseNotes(app, raw, testInternalSecret, now); rec.Code != http.StatusOK {
			t.Fatalf("seed %s: status=%d body=%s", seed.version, rec.Code, rec.Body.String())
		}
	}
	if err := app.db.Table("release_notes").Where("version = ?", "1.0.0").
		Update("created_at", now.UTC().Add(-2*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	rec := getLatestReleaseNote(app, testInternalSecret, time.Now())
	requireStatus(t, rec, http.StatusOK)
	view, ok := decodeData(t, rec).(map[string]any)
	if !ok {
		t.Fatalf("data=%v", decodeData(t, rec))
	}
	if view["version"] != "1.1.0" || view["release_sha"] != "bbb" {
		t.Fatalf("latest must be the newest note: %v", view)
	}
}

func TestListReleaseNotesIsPublicAndOrdered(t *testing.T) {
	app := newReleaseNotesTestApp(t, testInternalSecret)
	now := time.Now()
	if rec := postReleaseNotes(app,
		[]byte(`{"version":"1.0.0","releaseSha":"aaa","contentMd":"# one"}`),
		testInternalSecret, now); rec.Code != http.StatusOK {
		t.Fatalf("seed 1.0.0: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postReleaseNotes(app,
		[]byte(`{"version":"1.1.0","releaseSha":"bbb","contentMd":"# two"}`),
		testInternalSecret, now); rec.Code != http.StatusOK {
		t.Fatalf("seed 1.1.0: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// 签名窗口只有五分钟,排序差通过 created_at 回填制造;必须用 UTC,
	// sqlite 的 TEXT 时间比较不分时区,混入本地时区会打乱排序。
	if err := app.db.Table("release_notes").Where("version = ?", "1.0.0").
		Update("created_at", now.UTC().Add(-2*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	// 公开只读:无需 Bearer、无需 workspace 头即可访问。
	req := httptest.NewRequest(http.MethodGet, "/release-notes", nil)
	rec := httptest.NewRecorder()
	app.engine.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
	data, ok := decodeData(t, rec).([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data=%v", decodeData(t, rec))
	}
	newest, oldest := data[0].(map[string]any), data[1].(map[string]any)
	if newest["version"] != "1.1.0" || oldest["version"] != "1.0.0" {
		t.Fatalf("order must be newest first: %v then %v", newest, oldest)
	}
	for _, key := range []string{"version", "date", "content_md", "release_sha"} {
		if _, present := newest[key]; !present {
			t.Fatalf("field %q missing from %v", key, newest)
		}
	}
	if newest["release_sha"] != "bbb" || newest["content_md"] != "# two" {
		t.Fatalf("newest=%v", newest)
	}
	if date, _ := newest["date"].(string); len(date) != 10 || date[4] != '-' {
		t.Fatalf("date=%v, want YYYY-MM-DD", newest["date"])
	}
}
