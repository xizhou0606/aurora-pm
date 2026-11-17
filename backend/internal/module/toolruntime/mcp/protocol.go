// Package mcp adapts the canonical Tool Runtime to MCP Streamable HTTP.
// It owns protocol frames only; tool definitions and execution remain in the
// protocol-neutral toolruntime package.
package mcp

import (
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	ProtocolVersion       = "2025-06-18"
	legacyProtocolVersion = "2025-03-26"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *protocolError  `json:"error,omitempty"`
}

type protocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Annotations  ToolAnnotations `json:"annotations"`
	Meta         map[string]any  `json:"_meta"`
}

type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

type CallToolResult struct {
	Content           []TextContent   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
	Meta              map[string]any  `json:"_meta"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func projectTool(definition toolruntime.Definition) Tool {
	readOnly := definition.Risk == toolruntime.RiskRead
	return Tool{
		Name: definition.Name, Description: definition.Summary,
		InputSchema:  append(json.RawMessage(nil), definition.InputSchema...),
		OutputSchema: append(json.RawMessage(nil), definition.OutputSchema...),
		Annotations: ToolAnnotations{
			ReadOnlyHint: readOnly, DestructiveHint: !readOnly,
			IdempotentHint: readOnly, OpenWorldHint: false,
		},
		Meta: map[string]any{
			"pm/version": definition.Version, "pm/lifecycle": definition.Lifecycle,
			"pm/authorization": definition.Authorization, "pm/risk": definition.Risk,
			"pm/changeSet": definition.ChangeSet, "pm/uat": definition.UAT,
			"pm/auditEvent": definition.AuditEvent,
		},
	}
}

func projectChangeSetControl(definition toolruntime.ChangeSetControlDefinition) Tool {
	readOnly := definition.Risk == toolruntime.RiskRead
	return Tool{
		Name: definition.Name, Description: definition.Summary,
		InputSchema:  append(json.RawMessage(nil), definition.InputSchema...),
		OutputSchema: append(json.RawMessage(nil), definition.OutputSchema...),
		Annotations: ToolAnnotations{
			ReadOnlyHint: readOnly, DestructiveHint: !readOnly,
			IdempotentHint: definition.Idempotent, OpenWorldHint: false,
		},
		Meta: map[string]any{
			"pm/version": definition.Version, "pm/control": "changeset", "pm/risk": definition.Risk,
		},
	}
}

func projectCall(result toolruntime.InvocationResult) CallToolResult {
	output := append(json.RawMessage(nil), result.Output...)
	if result.ChangeSet != nil {
		output, _ = json.Marshal(map[string]any{"change_set": result.ChangeSet})
	}
	meta := map[string]any{
		"pm/name": result.Name, "pm/version": result.Version, "pm/auditEvent": result.AuditEvent,
	}
	if result.ChangeSet != nil {
		meta["pm/changeSetId"], meta["pm/changeSetStatus"] = result.ChangeSet.ID, result.ChangeSet.Status
	}
	return CallToolResult{
		Content:           []TextContent{{Type: "text", Text: string(output)}},
		StructuredContent: output, IsError: result.ChangeSet != nil && result.ChangeSet.Failed(),
		Meta: meta,
	}
}

func projectChangeSetControlCall(name string, changeSet toolruntime.ChangeSet) CallToolResult {
	output, _ := json.Marshal(changeSet)
	return CallToolResult{
		Content:           []TextContent{{Type: "text", Text: string(output)}},
		StructuredContent: output,
		IsError:           changeSet.Failed(),
		Meta: map[string]any{
			"pm/name": name, "pm/changeSetId": changeSet.ID, "pm/changeSetStatus": changeSet.Status,
		},
	}
}

func supportedProtocolVersion(version string) string {
	if version == ProtocolVersion || version == legacyProtocolVersion {
		return version
	}
	return ProtocolVersion
}
