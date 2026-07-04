package mcp

import (
	"encoding/json"
	"testing"
)

func TestCatalogCompleteness(t *testing.T) {
	tools := Catalog()
	byName := map[string]Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	expected := []string{
		"search", "files_read", "files_write", "exec",
		"browser_session_create", "browser_action",
	}
	for _, name := range expected {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("catalog missing tool %q", name)
			continue
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %q input schema type = %q, want object", name, tool.InputSchema.Type)
		}
		if tool.InputSchema.Properties == nil {
			t.Errorf("tool %q has nil properties", name)
		}
	}
}

func TestCatalogRequiredFields(t *testing.T) {
	byName := map[string]Tool{}
	for _, tool := range Catalog() {
		byName[tool.Name] = tool
	}

	cases := map[string][]string{
		"search":         {"query"},
		"files_read":     {"path"},
		"files_write":    {"path", "content"},
		"exec":           {"language", "code"},
		"browser_action": {"session_id", "type"},
	}
	for name, req := range cases {
		tool := byName[name]
		got := map[string]bool{}
		for _, r := range tool.InputSchema.Required {
			got[r] = true
		}
		for _, r := range req {
			if !got[r] {
				t.Errorf("tool %q missing required field %q", name, r)
			}
		}
	}
}

func TestCatalogSchemaMarshals(t *testing.T) {
	for _, tool := range Catalog() {
		if _, err := json.Marshal(tool); err != nil {
			t.Errorf("tool %q failed to marshal: %v", tool.Name, err)
		}
	}
}

func TestInitializeResponseShape(t *testing.T) {
	resp := InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
		},
		ServerInfo: ServerInfo{Name: "toolset-api", Version: "test"},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion mismatch: %v", back["protocolVersion"])
	}
	if _, ok := back["capabilities"]; !ok {
		t.Errorf("capabilities missing from serialized response")
	}
}

func TestToolsCallRequestParsesRawArguments(t *testing.T) {
	body := `{"params":{"name":"search","arguments":{"query":"x","page":2}}}`
	var req ToolsCallRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Params.Name != "search" {
		t.Errorf("expected name search, got %s", req.Params.Name)
	}
	var args struct {
		Query string `json:"query"`
		Page  int    `json:"page"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		t.Fatalf("unmarshal arguments: %v", err)
	}
	if args.Query != "x" || args.Page != 2 {
		t.Errorf("unexpected args: %+v", args)
	}
}
