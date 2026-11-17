package requirement

import "context"

// DocumentParticipants is the narrow requirement projection used by the
// document module's consumer-side resolver. It contains no GORM record and no
// authorization conclusion.
type DocumentParticipants struct {
	RequirementID        uint
	WorkspaceID          uint
	Version              uint
	Active               bool
	ProductOwnerUserIDs  []uint
	DesignOwnerUserIDs   []uint
	FrontendOwnerUserIDs []uint
	BackendOwnerUserIDs  []uint
	TesterUserIDs        []uint
	DeveloperUserIDs     []uint
}

type DocumentParticipantService struct{ store ReadStore }

func NewDocumentParticipantService(store ReadStore) (*DocumentParticipantService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &DocumentParticipantService{store: store}, nil
}

func (service *DocumentParticipantService) ResolveDocumentParticipants(
	ctx context.Context,
	workspaceID uint,
	requirementID uint,
) (DocumentParticipants, error) {
	if workspaceID == 0 || requirementID == 0 {
		return DocumentParticipants{}, ErrReadQueryInvalid
	}
	snapshot, err := service.store.GetRequirement(ctx, workspaceID, requirementID)
	if err != nil {
		return DocumentParticipants{}, err
	}
	return DocumentParticipants{
		RequirementID: snapshot.ID, WorkspaceID: snapshot.WorkspaceID, Version: snapshot.Version, Active: true,
		ProductOwnerUserIDs:  append([]uint(nil), snapshot.ProductOwnerUserIDs...),
		DesignOwnerUserIDs:   append([]uint(nil), snapshot.DesignOwnerUserIDs...),
		FrontendOwnerUserIDs: append([]uint(nil), snapshot.FrontendOwnerUserIDs...),
		BackendOwnerUserIDs:  append([]uint(nil), snapshot.BackendOwnerUserIDs...),
		TesterUserIDs:        append([]uint(nil), snapshot.TesterUserIDs...),
		DeveloperUserIDs:     append([]uint(nil), snapshot.DeveloperUserIDs...),
	}, nil
}
