package audit

import (
	"context"
	"errors"
	"strings"
)

var ErrMutationEffectInvalid = errors.New("audit mutation effect is invalid")

type MutationEffect struct {
	WorkspaceID  uint
	PrincipalID  uint
	ProjectSetID uint
	Resource     string
	ResourceID   uint
	ResourceRef  string
	Action       string
	Target       string
	EventType    string
	Payload      map[string]any
}

type MutationEffectStore interface {
	RecordMutationEffect(context.Context, MutationEffect) error
}

type MutationEffectService struct{ store MutationEffectStore }

func NewMutationEffectService(store MutationEffectStore) (*MutationEffectService, error) {
	if store == nil {
		return nil, ErrMutationEffectInvalid
	}
	return &MutationEffectService{store: store}, nil
}

func (service *MutationEffectService) RecordMutationEffect(ctx context.Context, effect MutationEffect) error {
	if service == nil || service.store == nil || effect.WorkspaceID == 0 || effect.PrincipalID == 0 ||
		(effect.ResourceID == 0 && effect.ResourceRef == "") || !validResourceRef(effect.ResourceRef) || strings.TrimSpace(effect.Resource) == "" || strings.TrimSpace(effect.Action) == "" ||
		strings.TrimSpace(effect.EventType) == "" || effect.Payload == nil {
		return ErrMutationEffectInvalid
	}
	return service.store.RecordMutationEffect(ctx, effect)
}

func validResourceRef(ref string) bool {
	if len(ref) > 128 {
		return false
	}
	for _, r := range ref {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' || r == ':') {
			return false
		}
	}
	return true
}
