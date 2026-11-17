package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBitableReferenceSchema(t *testing.T) {
	body := `{"type":"doc","content":[{"type":"bitable","attrs":{"schemaVersion":1,"resourceId":"00000000-0000-4000-8000-000000000001","tableId":"00000000-0000-4000-8000-000000000002","viewId":"00000000-0000-4000-8000-000000000003","height":420}}]}`
	if _, _, _, err := NormalizeContentForSchema(json.RawMessage(body), SchemaVersionV3); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := NormalizeContentForSchema(json.RawMessage(body), SchemaVersionV2); err == nil {
		t.Fatal("v2 accepted bitable")
	}
	for _, bad := range []string{strings.Replace(body, `"height":420`, `"height":9000`, 1), strings.Replace(body, `"height":420`, `"height":420,"token":"secret"`, 1), strings.Replace(body, `"schemaVersion":1`, `"schemaVersion":2`, 1)} {
		if _, _, _, err := NormalizeContentForSchema(json.RawMessage(bad), SchemaVersionV3); err == nil {
			t.Fatal("invalid reference accepted")
		}
	}
}
