package bitable

import "context"

type ResourceMutation struct {
	OperationID      string `json:"operationId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
	Name             string `json:"name,omitempty"`
	ParentFolderID   *uint  `json:"parentFolderId"`
	OwnerUserID      uint   `json:"ownerUserId,omitempty"`
	Pinned           *bool  `json:"pinned,omitempty"`
	Favorited        *bool  `json:"favorited,omitempty"`
}
type ResourcePreference struct {
	Revision          uint64
	Pinned, Favorited bool
}
type Shortcut struct {
	ID               string `json:"id"`
	TargetResourceID string `json:"targetResourceId"`
	ParentFolderID   *uint  `json:"parentFolderId"`
	Revision         uint64 `json:"revision"`
	Name             string `json:"name"`
}
type ResourceManager interface {
	MutateResource(context.Context, Principal, string, string, ResourceMutation) (Resource, error)
	ResourcePreference(context.Context, Principal, string) (ResourcePreference, error)
	Shortcuts(context.Context, Principal) ([]Shortcut, error)
	GetShortcut(context.Context, Principal, string) (Shortcut, error)
	MutateShortcut(context.Context, Principal, string, string, ResourceMutation) (Shortcut, error)
}

// FolderDirectory is implemented by a public document service adapter. The
// bitable repository never reads document tables or creates a second folder tree.
type FolderDirectory interface {
	ValidateBitableFolder(context.Context, Principal, uint) error
}

func (s *Service) ConfigureFolderDirectory(d FolderDirectory) { s.folders = d }
func (s *Service) decorateResource(ctx context.Context, p Principal, v Resource, binding ResourceBinding) (Resource, error) {
	if m, ok := s.store.(ResourceManager); ok {
		pref, e := m.ResourcePreference(ctx, p, binding.NativeResourceID)
		if e != nil {
			return v, e
		}
		v.Pinned = pref.Pinned
		v.Favorited = pref.Favorited
		v.PreferenceRevision = pref.Revision
	}
	v = projectResource(v, binding)
	v.AllowedActions = s.allowedActions(ctx, p, binding)
	return s.projectDirectoryLocation(ctx, p, v, binding)
}
