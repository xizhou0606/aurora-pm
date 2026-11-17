package workbenchhttp

import (
	"bytes"
	"encoding/json"
)

type createMenuRequest struct {
	Code      string `json:"code"`
	ParentID  *uint  `json:"parent_id"`
	Name      string `json:"name"`
	MenuType  string `json:"menu_type"`
	Path      string `json:"path"`
	Icon      string `json:"icon"`
	HostKey   string `json:"host_key"`
	SortOrder int    `json:"sort_order"`
	Enabled   *bool  `json:"enabled"`
}

type patchMenuRequest struct {
	Code      *string          `json:"code"`
	ParentID  optionalParentID `json:"parent_id"`
	Name      *string          `json:"name"`
	MenuType  *string          `json:"menu_type"`
	Path      *string          `json:"path"`
	Icon      *string          `json:"icon"`
	HostKey   *string          `json:"host_key"`
	SortOrder *int             `json:"sort_order"`
	Enabled   *bool            `json:"enabled"`
}

// optionalParentID preserves the important distinction between an omitted
// parent and an explicit JSON null that detaches a menu from its parent.
type optionalParentID struct {
	Set   bool
	Value *uint
}

func (value *optionalParentID) UnmarshalJSON(raw []byte) error {
	value.Set = true
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		value.Value = nil
		return nil
	}
	var id uint
	if err := json.Unmarshal(raw, &id); err != nil {
		return err
	}
	value.Value = &id
	return nil
}
