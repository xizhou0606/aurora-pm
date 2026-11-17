package identity

import "context"

// UserDisplay is a minimal profile for an already-authorized object's user references.
// IDs are PM primary keys, never external/fact identifiers or membership grants.
type UserDisplay struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}
type UserDisplayReader interface {
	ReadUserDisplays(context.Context, []uint) (map[uint]UserDisplay, error)
}
