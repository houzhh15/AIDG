package main

import (
	"testing"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

type schemaTestTool struct{ name string }

func (t schemaTestTool) Name() string        { return t.name }
func (t schemaTestTool) Description() string { return "test" }
func (t schemaTestTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{"type": "string"},
			"task_id":    map[string]interface{}{"type": "string"},
		},
		"required": []string{},
	}
}
func (t schemaTestTool) Execute(map[string]interface{}, string, *shared.APIClient) (string, error) {
	return "", nil
}

func TestRegistryMakesDeclaredBusinessIDsRequired(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(schemaTestTool{name: "business_tool"})
	tool, err := registry.Get("business_tool")
	if err != nil {
		t.Fatal(err)
	}
	required := tool.InputSchema()["required"].([]string)
	seen := map[string]bool{}
	for _, field := range required {
		seen[field] = true
	}
	if !seen["project_id"] || !seen["task_id"] {
		t.Fatalf("business IDs not required: %#v", required)
	}
}

func TestRegistryKeepsDocumentTaskIDConditional(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(schemaTestTool{name: "read_document"})
	tool, err := registry.Get("read_document")
	if err != nil {
		t.Fatal(err)
	}
	required := tool.InputSchema()["required"].([]string)
	seen := map[string]bool{}
	for _, field := range required {
		seen[field] = true
	}
	if !seen["project_id"] || seen["task_id"] {
		t.Fatalf("document conditional IDs changed: %#v", required)
	}
}
