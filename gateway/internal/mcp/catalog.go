package mcp

// Catalog returns the full set of tools advertised via /mcp/tools/list, with
// complete JSON Schema input contracts. The names here are the canonical MCP
// tool identifiers dispatched by /mcp/tools/call.
func Catalog() []Tool {
	return []Tool{
		{
			Name:        "search",
			Description: "Web/meta search via the SearXNG aggregator.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"engines": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Optional engine allow-list (e.g. google, bing)",
					},
					"page": map[string]interface{}{
						"type":        "number",
						"description": "Result page (1-based)",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Optional language code (e.g. en)",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "files_read",
			Description: "Read a file from the sandbox filesystem.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path within the sandbox root",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "files_write",
			Description: "Write a file to the sandbox filesystem.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path within the sandbox root",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "File contents to write",
					},
				},
				Required: []string{"path", "content"},
			},
		},
		{
			Name:        "exec",
			Description: "Execute code in a sandboxed runtime and return stdout/stderr.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"language": map[string]interface{}{
						"type":        "string",
						"description": "Runtime: python, node, bash, c, cpp, assembly, java, rust, csharp, dotnet",
					},
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Source code to execute",
					},
					"stdin": map[string]interface{}{
						"type":        "string",
						"description": "Optional standard input",
					},
					"timeout": map[string]interface{}{
						"type":        "number",
						"description": "Optional wall-clock timeout in seconds",
					},
				},
				Required: []string{"language", "code"},
			},
		},
		{
			Name:        "browser_session_create",
			Description: "Create a browser automation session and return its id.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"browserType": map[string]interface{}{
						"type":        "string",
						"description": "chromium (default), firefox, or webkit",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:        "browser_action",
			Description: "Execute a DOM action within an existing browser session.",
			InputSchema: ToolInput{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Target browser session id",
					},
					"type": map[string]interface{}{
						"type": "string",
						"description": "Action type: navigate, click, type, eval, screenshot, " +
							"pdf, content, wait_for_selector, wait_for_navigation, " +
							"get_title, get_url, set_viewport",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Target URL (navigate)",
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector (click/type/wait_for_selector)",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to type (type)",
					},
					"script": map[string]interface{}{
						"type":        "string",
						"description": "JavaScript to evaluate (eval)",
					},
				},
				Required: []string{"session_id", "type"},
			},
		},
	}
}
