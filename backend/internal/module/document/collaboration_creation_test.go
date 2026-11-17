package document

import (
	"context"
	"testing"
)

func TestCollaborationCreationCapabilityIsAuthoritative(t *testing.T) {
	actions := []string{string(ActionNodesView), string(ActionNodesCreate), string(ActionContentView), string(ActionContentUpdate), string(ActionCollaborationView), string(ActionCollaborationEdit)}
	for _, missing := range append([]string{"", "disabled"}, actions[1:]...) {
		t.Run(missing, func(t *testing.T) {
			codes := []string{}
			for _, code := range actions {
				if code != missing {
					codes = append(codes, code)
				}
			}
			service, _ := newPolicyTestService(t, codes)
			service.ConfigureCollaborationCreation(missing != "disabled")
			space, err := service.DefaultSpace(context.Background(), Principal{WorkspaceID: 7, UserID: 5})
			if err != nil {
				t.Fatal(err)
			}
			if space.CollaborationEnabled != (missing == "") {
				t.Fatalf("unexpected capability: %v", space.CollaborationEnabled)
			}
			if missing != "" {
				_, err := service.CreateNode(context.Background(), Principal{WorkspaceID: 7, UserID: 5}, CreateNodeInput{Type: NodeDocument, Title: "Explicit", Collaborative: true})
				if err != ErrForbidden {
					t.Fatalf("explicit unauthorized collaboration: %v", err)
				}
			}
		})
	}
}
