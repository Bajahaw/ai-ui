package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// builtInToolsVersion is the human-readable version shown in the UI. Bump it
// whenever GetBuiltInTools changes; TestBuiltInToolsVersionIsCurrent fails
// until builtInToolsHash is updated with it.
//
// Syncing does not depend on remembering the bump: the stored marker also
// carries the content hash, so any change to the definitions reaches users.
const (
	builtInToolsVersion = 1
	builtInToolsHash    = "50e56c7527220a9158f1fb217345767ba2d06394a78da56d6d3246524ec8517f"
)

// GetBuiltInTools returns the platform built-in tools.
// MCPServerID is left empty; callers must set it to the owning server ID
// (e.g. "default-{user}") before persisting, so the Tools FK is satisfied.
func GetBuiltInTools() []*Tool {
	return []*Tool{
		{
			ID:          uuid.New().String(),
			Name:        "search_document",
			Description: "Search a specific attached document for a keyword or phrase constraint. Returns best matching pages.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"query":{"type":"string","description":"The keyword or phrase to search for"}},"required":["file_id","query"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "read_document_page",
			Description: "Read the extracted text of specific pages from a retreivable attached document.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"start_page":{"type":"integer","description":"The 1-based page number to start reading from"},"end_page":{"type":"integer","description":"The 1-based page number to end reading at (inclusive)"}},"required":["file_id","start_page","end_page"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "view_document_page",
			Description: "Get a screenshot of a specific PDF page (works only with pdf!). Use this when the user specifically mentions looking at an image, chart, format, or layout in a PDF. Pass array of files_ids via file_id property if needed.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"page_number":{"type":"integer","description":"The 1-based page number to view"}},"required":["file_id","page_number"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "generate_image",
			Description: "Generate an image via AI model currently selected by user. Pass the user prompt exactly as is, unless user requested you to enhance it. Embed the resulting image file in the chat. ONlY call when user asks for `AI generated image`!, and Never call more than once",
			InputSchema: `{"type":"object","properties":{"prompt":{"type":"string","description":"A detailed prompt for the image generation model"}},"required":["prompt"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "read_skill",
			Description: "Read the full content of a specific skill by its name. Choose the skill that best matches the user's task from the <available_skills> section in the system prompt, then read its full instructions using this tool.",
			InputSchema: `{"type":"object","properties":{"name":{"type":"string","description":"The exact name of the skill to read"}},"required":["name"]}`,
			IsEnabled:   true,
		},
		{
			// Off by default and approval-gated: executing code should be an
			// explicit user opt-in, not something that runs unnoticed.
			ID:              uuid.New().String(),
			Name:            "browser_sandbox",
			Description:     "Run JavaScript in an isolated browser sandbox with the conversation's files mounted. Top-level await is allowed; the code's return value (JSON-serialized), console output, and any files written are reported back — return or console.log anything you need to see. API: sandbox.listFiles() -> names; sandbox.readFile(name) -> Uint8Array; sandbox.writeFile(name, data, mime) saves an output file; await sandbox.loadScript(url) loads a classic <script> library and rejects fast on a bad URL/blocked host (only https://cdn.jsdelivr.net and https://cdnjs.cloudflare.com are allowed; ESM builds can also be loaded via dynamic import()). Not HTML: there is no visible page, so do not build documents/script tags as strings. When embedding other languages (e.g. Python source) in a template literal, use String.raw and avoid nested backticks.",
			InputSchema:     `{"type":"object","properties":{"code":{"type":"string","description":"JavaScript source to run (async context; top-level await and return are allowed)."}},"required":["code"]}`,
			RequireApproval: true,
		},
		{
			ID:              uuid.New().String(),
			Name:            "http_request",
			Description:     "HTTPS request to a public host (no IPs/private/loopback). Prefer json_pointers or css_selectors to extract fields. Response bodies are reduced by default; verbose for raw/larger. Secrets: $secrets.NAME$ in headers or URL path/query only.",
			InputSchema:     `{"type":"object","properties":{"url":{"type":"string","description":"https URL (DNS hostname). $secrets.NAME$ in path/query."},"method":{"type":"string","description":"GET, HEAD, POST, PUT, PATCH, or DELETE","default":"GET"},"headers":{"type":"object","additionalProperties":{"type":"string"},"description":"Optional headers. $secrets.NAME$ allowed."},"body":{"description":"Optional body for POST/PUT/PATCH/DELETE. Prefer JSON object/array."},"verbose":{"type":"boolean","description":"Raw/larger response body.","default":false},"json_pointers":{"type":"array","items":{"type":"string"},"description":"RFC 6901 pointers into JSON body (e.g. \"/data/0/id\"). Missing paths → null. Exclusive with css_selectors."},"css_selectors":{"type":"array","items":{"type":"string"},"description":"CSS selectors into HTML (e.g. \"a.result\"). Exclusive with json_pointers."}},"required":["url"]}`,
			RequireApproval: true,
			IsEnabled:       true,
		},
	}
}

// isBuiltInServer matches the per-user default server ("default-{user}", or
// "default_{user}" from older migrations).
func isBuiltInServer(id string) bool {
	return strings.HasPrefix(id, "default")
}

// builtInToolsFor returns fresh built-in tools owned by serverID.
func builtInToolsFor(serverID string) []*Tool {
	tools := GetBuiltInTools()
	for _, t := range tools {
		t.MCPServerID = serverID
	}
	return tools
}

// hashBuiltInTools hashes what the model and the user-facing defaults see:
// names, descriptions, schemas and the enabled/approval defaults. IDs are
// random, so they are left out.
var hashBuiltInTools = sync.OnceValue(func() string {
	type entry struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		InputSchema     string `json:"input_schema"`
		IsEnabled       bool   `json:"is_enabled"`
		RequireApproval bool   `json:"require_approval"`
	}
	tools := GetBuiltInTools()
	entries := make([]entry, len(tools))
	for i, t := range tools {
		entries[i] = entry{t.Name, t.Description, t.InputSchema, t.IsEnabled, t.RequireApproval}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	b, _ := json.Marshal(entries)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
})

// builtInServerInfo is stored on the default server like an MCP server's
// initialize info. Version uses semver build metadata ("v1+<hash>"): the
// display shows "v1", and the whole string is the sync marker.
func builtInServerInfo() MCPServerInfo {
	return MCPServerInfo{
		Name:        "ai-ui",
		Title:       "Built-in tools",
		Version:     fmt.Sprintf("v%d+%s", builtInToolsVersion, hashBuiltInTools()[:12]),
		Description: "Tools built into ai-ui: document search and reading, image generation, skills, HTTP requests and the browser sandbox.",
	}
}

// syncBuiltInServer brings a default server's stored tools up to the current
// built-in definitions when its marker is out of date. It reports whether
// anything was written. User-set enabled/approval flags are kept.
func syncBuiltInServer(server *MCPServer) (bool, error) {
	info := builtInServerInfo()
	if server.Info == info {
		return false, nil
	}
	if err := syncTools(server.ID, builtInToolsFor(server.ID)); err != nil {
		return false, err
	}
	return true, mcps.UpdateServerInfo(server.ID, server.User, info)
}
