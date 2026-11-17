package mcp

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"strings"
	"testing"
)

func TestChangeSetBusinessFailureFlags(t *testing.T) {
	for _, status := range []toolruntime.ChangeSetStatus{toolruntime.ChangeSetFailed, toolruntime.ChangeSetRejected, toolruntime.ChangeSetExpired, toolruntime.ChangeSetPending, toolruntime.ChangeSetConfirmed, toolruntime.ChangeSetSucceeded} {
		t.Run(string(status), func(t *testing.T) {
			cs := toolruntime.ChangeSet{Status: status, ErrorCode: "DOCUMENT_EDITING_IN_PROGRESS"}
			expected := status == toolruntime.ChangeSetFailed || status == toolruntime.ChangeSetRejected || status == toolruntime.ChangeSetExpired
			for _, result := range []CallToolResult{projectChangeSetControlCall("changeset.confirm", cs), projectCall(toolruntime.InvocationResult{ChangeSet: &cs})} {
				if result.IsError != expected {
					t.Fatalf("isError=%v want=%v", result.IsError, expected)
				}
				if !strings.Contains(string(result.StructuredContent), "DOCUMENT_EDITING_IN_PROGRESS") {
					t.Fatal("business code lost")
				}
			}
		})
	}
}
