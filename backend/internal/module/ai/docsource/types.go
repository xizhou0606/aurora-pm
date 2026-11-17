// Package docsource owns the deterministic document input stage before model execution.
package docsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const ManifestKey = "_document_sources"
const Feishu = "feishu_link"
const Internal = "internal_document"

var ErrUnavailable = errors.New("文档来源不可用或权限已失效，请检查关联后重新生成")
var ErrChanged = errors.New("文档或需求关联在生成期间发生变化，请重新生成")

type ReadContext struct {
	WorkspaceID uint   `json:"workspace_id"`
	UserID      uint   `json:"user_id"`
	FactUserID  uint   `json:"fact_user_id"`
	Method      string `json:"method"`
	Route       string `json:"route"`
}
type RequirementRef struct {
	ID        uint `json:"id"`
	Version   uint `json:"version"`
	ProjectID uint `json:"project_id"`
}
type SourceRef struct {
	ConverterVersion string           `json:"converter_version,omitempty"`
	Kind             string           `json:"kind"`
	ResourceID       string           `json:"resource_id,omitempty"`
	URL              string           `json:"url,omitempty"`
	Requirements     []RequirementRef `json:"requirements"`
}
type Manifest struct {
	Version      int              `json:"version"`
	Context      ReadContext      `json:"context"`
	Requirements []RequirementRef `json:"requirements"`
	Sources      []SourceRef      `json:"sources"`
}
type Snapshot struct {
	Type                 string   `json:"type"`
	URL                  string   `json:"url,omitempty"`
	ResourceID           string   `json:"resource_id,omitempty"`
	RequirementIDs       []uint   `json:"requirement_ids"`
	Title                string   `json:"title"`
	Content              string   `json:"content"`
	Format               string   `json:"content_format"`
	Revision             string   `json:"revision"`
	ContentHash          string   `json:"content_hash"`
	ProjectionHash       string   `json:"projection_hash,omitempty"`
	ConverterVersion     string   `json:"converter_version,omitempty"`
	BlockRefs            []string `json:"block_refs"`
	Unexpanded           []string `json:"unexpanded_resources,omitempty"`
	Available            bool     `json:"available"`
	OriginalContentChars int      `json:"original_content_chars"`
	ContentTruncated     bool     `json:"content_truncated"`
	BlockRefsTruncated   bool     `json:"block_refs_truncated"`
}

type Provider interface {
	Kind() string
	Read(context.Context, ReadContext, SourceRef) (Snapshot, error)
}
type Registry struct{ providers map[string]Provider }

func NewRegistry(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: map[string]Provider{}}
	for _, p := range providers {
		if p == nil || p.Kind() == "" || r.providers[p.Kind()] != nil {
			return nil, fmt.Errorf("invalid or duplicate document provider")
		}
		r.providers[p.Kind()] = p
	}
	return r, nil
}
func (r *Registry) Read(ctx context.Context, actor ReadContext, ref SourceRef) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if r == nil || r.providers[ref.Kind] == nil {
		return Snapshot{}, ErrUnavailable
	}
	snapshot, err := r.providers[ref.Kind].Read(ctx, actor, ref)
	if err != nil {
		return Snapshot{}, err
	}
	for _, req := range ref.Requirements {
		snapshot.RequirementIDs = append(snapshot.RequirementIDs, req.ID)
	}
	return snapshot, nil
}
func DecodeManifest(input map[string]any) (*Manifest, error) {
	value, ok := input[ManifestKey]
	if !ok {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, ErrUnavailable
	}
	var m Manifest
	if json.Unmarshal(raw, &m) != nil || m.Version != 1 || m.Context.UserID == 0 || m.Context.FactUserID == 0 || m.Context.WorkspaceID == 0 || m.Context.Method == "" || m.Context.Route == "" {
		return nil, ErrUnavailable
	}
	return &m, nil
}
