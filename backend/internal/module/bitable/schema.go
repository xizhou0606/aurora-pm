package bitable

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

type FieldMutation struct {
	Name                   string         `json:"name"`
	Type                   FieldType      `json:"type"`
	Options                FieldOptions   `json:"options"`
	ExpectedSchemaRevision uint64         `json:"expectedSchemaRevision"`
	OperationID            string         `json:"operationId"`
	CellContracts          []CellContract `json:"cellContracts,omitempty"`
}
type ViewMutation struct {
	Name             string     `json:"name"`
	Kind             string     `json:"kind"`
	Config           ViewConfig `json:"config"`
	ExpectedRevision *uint64    `json:"expectedRevision,omitempty"`
	OperationID      string     `json:"operationId"`
}
type SchemaProvider interface {
	MutateField(context.Context, Scope, string, FieldMutation) (Field, error)
	MutateView(context.Context, Scope, string, ViewMutation) (View, error)
}

func (s *Service) Field(ctx context.Context, p Principal, ref, table, id string, in FieldMutation) (Field, error) {
	if e := ValidateFieldMutation(in); e != nil {
		return Field{}, e
	}
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationStructure)
	if e != nil {
		return Field{}, e
	}
	provider, ok := s.provider.(SchemaProvider)
	if !ok {
		return Field{}, ErrUnavailable
	}
	return provider.MutateField(ctx, scope, id, in)
}
func (s *Service) View(ctx context.Context, p Principal, ref, table, id string, in ViewMutation) (View, error) {
	if e := ValidateViewMutation(in, id != ""); e != nil {
		return View{}, e
	}
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationStructure)
	if e != nil {
		return View{}, e
	}
	provider, ok := s.provider.(SchemaProvider)
	if !ok {
		return View{}, ErrUnavailable
	}
	return provider.MutateView(ctx, scope, id, in)
}
