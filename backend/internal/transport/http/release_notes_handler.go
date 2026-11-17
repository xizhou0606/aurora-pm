package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	releasenotes "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes"
	releasenotesgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes/gorm"
)

// internalSignatureMaxSkew matches the ai-runtime validSignature window
// (apps/ai-runtime/src/app.ts): requests older or fresher than five minutes
// against the x-pm-timestamp unix-millisecond value are rejected.
const internalSignatureMaxSkew = 5 * time.Minute

// releaseNotesBodyLimit bounds the internal write payload. Release notes are
// per-version markdown documents, so 1 MiB is a generous ceiling.
const releaseNotesBodyLimit = 1 << 20

func newReleaseNotesService(db *gorm.DB) *releasenotes.Service {
	return releasenotes.NewService(releasenotesgorm.NewStore(db))
}

// verifyInternalSignature mirrors the ai-runtime internal call scheme used by
// the AI worker (internal/worker/ai/worker.go sign): HMAC-SHA256 over
// `${timestamp}.${rawBody}` with the AI_INTERNAL_SECRET, hex encoded, carried
// in X-PM-Timestamp / X-PM-Signature. It fails closed when the secret is not
// configured.
func (s *Server) verifyInternalSignature(rawBody []byte, timestamp, signature string) bool {
	secret := strings.TrimSpace(s.cfg.AI.InternalSecret)
	if secret == "" {
		return false
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if skew := time.Since(time.UnixMilli(unix)); skew > internalSignatureMaxSkew || skew < -internalSignatureMaxSkew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(rawBody)
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(mac.Sum(nil), provided)
}

type releaseNoteWrite struct {
	Version    string `json:"version"`
	ReleaseSHA string `json:"releaseSha"`
	ContentMd  string `json:"contentMd"`
	Source     string `json:"source"`
}

// createReleaseNote serves POST /internal/release-notes for the deployment
// pipeline: HMAC-authenticated, idempotent per version + release SHA.
func (s *Server) createReleaseNote(c *gin.Context) {
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, releaseNotesBodyLimit))
	if err != nil {
		bad(c, "unreadable request body")
		return
	}
	if !s.verifyInternalSignature(raw, c.GetHeader("X-PM-Timestamp"), c.GetHeader("X-PM-Signature")) {
		fail(c, http.StatusUnauthorized, "invalid internal signature")
		return
	}
	var payload releaseNoteWrite
	if err := json.Unmarshal(raw, &payload); err != nil {
		bad(c, "invalid JSON body")
		return
	}
	note, err := s.releaseNotes.Publish(c.Request.Context(), releasenotes.PublishInput{
		Version: payload.Version, ReleaseSHA: payload.ReleaseSHA,
		ContentMd: payload.ContentMd, Source: payload.Source,
	})
	switch {
	case errors.Is(err, releasenotes.ErrInvalidInput):
		bad(c, "version and contentMd are required")
		return
	case errors.Is(err, releasenotes.ErrVersionConflict):
		failCode(c, http.StatusConflict, "release_note_version_conflict",
			"release note version already exists with a different release sha")
		return
	case err != nil:
		dbFail(c)
		return
	}
	ok(c, releaseNoteView(note))
}

// latestReleaseNote serves GET /internal/release-notes/latest for the
// deployment launcher: HMAC-authenticated read of the newest stored note so
// the next announcement anchors on the version + release SHA pm-api already
// has instead of the frozen CHANGELOG.md in the read-only repo mount. GET has
// no body, so the signature covers `${timestamp}.` only.
func (s *Server) latestReleaseNote(c *gin.Context) {
	if !s.verifyInternalSignature(nil, c.GetHeader("X-PM-Timestamp"), c.GetHeader("X-PM-Signature")) {
		fail(c, http.StatusUnauthorized, "invalid internal signature")
		return
	}
	note, found, err := s.releaseNotes.Latest(c.Request.Context())
	if err != nil {
		dbFail(c)
		return
	}
	if !found {
		ok(c, nil)
		return
	}
	ok(c, releaseNoteView(note))
}

// listReleaseNotes serves GET /release-notes: a public read-only feed newest
// first for the release announcement page.
func (s *Server) listReleaseNotes(c *gin.Context) {
	notes, err := s.releaseNotes.List(c.Request.Context())
	if err != nil {
		dbFail(c)
		return
	}
	items := make([]gin.H, 0, len(notes))
	for _, note := range notes {
		items = append(items, releaseNoteView(note))
	}
	ok(c, items)
}

func releaseNoteView(note releasenotes.Note) gin.H {
	return gin.H{
		"version":     note.Version,
		"date":        note.CreatedAt.UTC().Format("2006-01-02"),
		"content_md":  note.ContentMd,
		"release_sha": note.ReleaseSHA,
	}
}
