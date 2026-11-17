package httpapi

import (
	"sort"
	"strings"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func selectRequirementPoolTestCaseArtifacts(
	definition workflowDefinition,
	selected map[string]bool,
	artifacts []workflowgorm.WorkflowNodeArtifact,
) []workflowgorm.WorkflowNodeArtifact {
	testCaseNodes := map[string]bool{}
	for _, node := range definition.Nodes {
		if selected[node.ID] && workflowNodeSupportsTestCases(node) {
			testCaseNodes[node.ID] = true
		}
	}
	var uploaded []workflowgorm.WorkflowNodeArtifact
	var generated []workflowgorm.WorkflowNodeArtifact
	for _, artifact := range artifacts {
		if !testCaseNodes[artifact.NodeID] {
			continue
		}
		switch artifact.Kind {
		case "uploaded":
			uploaded = append(uploaded, artifact)
		case "ai_generated":
			generated = append(generated, artifact)
		}
	}
	if len(uploaded) > 0 {
		sort.Slice(uploaded, func(i, j int) bool { return uploaded[i].ID < uploaded[j].ID })
		return uploaded
	}
	sort.Slice(generated, func(i, j int) bool { return generated[i].ID < generated[j].ID })
	return generated
}

func selectRequirementPoolTestCaseArtifact(
	definition workflowDefinition,
	selected map[string]bool,
	artifacts []workflowgorm.WorkflowNodeArtifact,
) *workflowgorm.WorkflowNodeArtifact {
	chosen := selectRequirementPoolTestCaseArtifacts(definition, selected, artifacts)
	if len(chosen) == 0 {
		return nil
	}
	return &chosen[len(chosen)-1]
}

func selectRequirementPoolDesignURLs(
	definition workflowDefinition,
	selected map[string]bool,
	runs []workflowgorm.WorkflowNodeRun,
	fallback string,
) []string {
	designNodes := map[string]bool{}
	designNodeCount := 0
	for _, node := range definition.Nodes {
		if selected[node.ID] && workflowNodeRequiresDesignAsset(definition, node) {
			designNodes[node.ID] = true
			designNodeCount++
		}
	}
	seen := map[string]bool{}
	urls := make([]string, 0)
	for _, run := range runs {
		if !designNodes[run.NodeID] {
			continue
		}
		url := strings.TrimSpace(decodeWorkflowRunActor(run.OutputJSON).DesignURL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		urls = append(urls, url)
	}
	if len(urls) > 0 {
		return urls
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" && designNodeCount <= 1 {
		return []string{fallback}
	}
	return nil
}

func mergeRequirementPoolDesignURLs(existing []string, extra []string) []string {
	seen := map[string]bool{}
	urls := make([]string, 0, len(existing)+len(extra))
	for _, value := range append(append([]string{}, existing...), extra...) {
		url := strings.TrimSpace(value)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		urls = append(urls, url)
	}
	return urls
}

func mergeRequirementPoolDesignArtifacts(
	existing []*requirementPoolDesignArtifact,
	urls []string,
) []*requirementPoolDesignArtifact {
	seen := map[string]bool{}
	artifacts := make([]*requirementPoolDesignArtifact, 0, len(existing)+len(urls))
	for _, item := range existing {
		if item == nil {
			continue
		}
		url := strings.TrimSpace(item.URL)
		if url != "" {
			seen[url] = true
		}
		artifacts = append(artifacts, item)
	}
	for _, value := range urls {
		url := strings.TrimSpace(value)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		artifacts = append(artifacts, &requirementPoolDesignArtifact{Name: url, URL: url})
	}
	return artifacts
}

func requirementPoolTestCaseArtifactDTO(
	artifact *workflowgorm.WorkflowNodeArtifact,
) *requirementPoolTestCaseArtifact {
	if artifact == nil {
		return nil
	}
	return &requirementPoolTestCaseArtifact{
		ID: artifact.ID, InstanceID: artifact.InstanceID, NodeID: artifact.NodeID,
		Kind: artifact.Kind, Name: artifact.Name, ContentType: artifact.ContentType,
		Size: artifact.Size, AIRunID: artifact.AIRunID,
		CreatedAt: artifact.CreatedAt,
	}
}

func requirementPoolTestCaseArtifactDTOs(
	artifacts []workflowgorm.WorkflowNodeArtifact,
) []*requirementPoolTestCaseArtifact {
	if len(artifacts) == 0 {
		return nil
	}
	result := make([]*requirementPoolTestCaseArtifact, 0, len(artifacts))
	for index := range artifacts {
		result = append(result, requirementPoolTestCaseArtifactDTO(&artifacts[index]))
	}
	return result
}
