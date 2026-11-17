package document

import (
	"context"
	"time"
)

type Service struct {
	contentReplacer              ContentReplacer
	store                        Store
	actions                      ActionResolver
	requirements                 RequirementParticipantResolver
	effects                      MutationRecorder
	assetStorage                 AssetStorage
	assetPrefix                  string
	comments                     CommentStore
	mentions                     CommentMentionResolver
	workspaceUsers               WorkspaceUserResolver
	resourceAccess               ResourceAccessResolver
	collaboration                CollaborationProjectionReader
	collaborationCreationEnabled bool
	now                          func() time.Time
}

func (service *Service) ConfigureCollaborationCreation(enabled bool) {
	service.collaborationCreationEnabled = enabled
}

func (service *Service) canCreateCollaboratively(state accessState) bool {
	return service.collaborationCreationEnabled && state.has(string(ActionNodesCreate)) &&
		state.has(string(ActionContentView)) && state.has(string(ActionContentUpdate)) &&
		state.has(string(ActionCollaborationView)) && state.has(string(ActionCollaborationEdit))
}

func (service *Service) ConfigureAssetStorage(storage AssetStorage, prefix string) {
	service.assetStorage = storage
	service.assetPrefix = prefix
}

func (service *Service) ConfigureComments(store CommentStore, mentions CommentMentionResolver) {
	service.comments = store
	service.mentions = mentions
}

func (service *Service) ConfigureWorkspaceUsers(users WorkspaceUserResolver) {
	service.workspaceUsers = users
}

func (service *Service) ConfigureResourceAccess(resolver ResourceAccessResolver) {
	service.resourceAccess = resolver
}

func (service *Service) ConfigureCollaborationProjection(reader CollaborationProjectionReader) {
	service.collaboration = reader
}

func NewService(
	store Store,
	actions ActionResolver,
	requirements RequirementParticipantResolver,
	effects MutationRecorder,
) (*Service, error) {
	if store == nil || actions == nil || requirements == nil {
		return nil, ErrInvalidInput
	}
	return &Service{
		store: store, actions: actions, requirements: requirements, effects: effects,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (service *Service) record(ctx context.Context, effect MutationEffect) error {
	if service.effects == nil {
		return nil
	}
	return service.effects.RecordDocumentMutation(ctx, effect)
}
