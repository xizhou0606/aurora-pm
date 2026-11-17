package api

import (
	"testing"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func TestDocumentDefaultGroupActionCodesIncludesRegisteredBindingParticipantCeiling(t *testing.T) {
	interfaces := []identity.ServiceInterfaceInput{
		{ActionCode: string(accesscatalog.ActionDocumentsNodesView)},
		{ActionCode: string(accesscatalog.ActionDocumentsContentView)},
		{ActionCode: string(accesscatalog.ActionDocumentsAssetsView)},
		{ActionCode: string(accesscatalog.ActionDocumentsAssetsUpload)},
		{ActionCode: string(accesscatalog.ActionDocumentsAssetsDownload)},
		{ActionCode: string(accesscatalog.ActionDocumentsAssetsDelete)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsView)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsCreate)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsReply)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsEditOwn)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsDeleteOwn)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsResolve)},
		{ActionCode: string(accesscatalog.ActionDocumentsCommentsReopen)},
		{ActionCode: string(accesscatalog.ActionDocumentsPreferencesUpdate)},
	}
	codes := documentDefaultGroupActionCodes(interfaces)
	for _, expected := range []string{
		string(accesscatalog.ActionDocumentsNodesView),
		string(accesscatalog.ActionDocumentsContentView),
		string(accesscatalog.ActionDocumentsAssetsView),
		string(accesscatalog.ActionDocumentsAssetsUpload),
		string(accesscatalog.ActionDocumentsAssetsDownload),
		string(accesscatalog.ActionDocumentsAssetsDelete),
		string(accesscatalog.ActionDocumentsCommentsView),
		string(accesscatalog.ActionDocumentsCommentsCreate),
		string(accesscatalog.ActionDocumentsCommentsReply),
		string(accesscatalog.ActionDocumentsCommentsEditOwn),
		string(accesscatalog.ActionDocumentsCommentsDeleteOwn),
		string(accesscatalog.ActionDocumentsCommentsResolve),
		string(accesscatalog.ActionDocumentsCommentsReopen),
		string(accesscatalog.ActionDocumentsPreferencesUpdate),
	} {
		if !containsString(codes, expected) {
			t.Fatalf("missing %s in %v", expected, codes)
		}
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
