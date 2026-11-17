package taskgorm

import (
	"os"
	"strings"
	"testing"
)

func TestTaskBatchMutationAdapterDoesNotImportForeignStorageOrTransport(t *testing.T) {
	files := []string{
		"batch_mutation_create.go", "batch_mutation_record.go", "batch_mutation_repository.go",
		"batch_mutation_selector.go", "batch_mutation_update.go",
	}
	for _, name := range files {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"internal/module/project/gorm", "internal/module/requirement/gorm", "internal/module/identity/gorm",
			"internal/module/notification/gorm", "internal/module/audit/gorm", "internal/transport/http", "gin-gonic/gin",
		} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("%s imports forbidden adapter %q", name, forbidden)
			}
		}
	}
}
