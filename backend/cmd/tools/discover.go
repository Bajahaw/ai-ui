package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/providers"
)

// Servers with more enabled tools than this are "deferred": their schemas
// are left out of the request's tool list (hundreds of schemas would
// dominate the prompt) and the model reaches them through list_tools /
// get_tool_schema, or sandbox.callTool inside the browser sandbox. Direct
// calls keep working regardless - execution looks tools up in the database,
// not in the prompt - and the system prompt lists their tool names.
const (
	promptDeferThreshold = 5
	promptNameLimit      = 60
	listToolsDefault     = 100
	listToolsMax         = 500
	listToolsDescRunes   = 200
	getSchemaMaxNames    = 20
	getSchemaMaxRunes    = 8000
)

// DeferredServerSummary describes one deferred server for the system prompt.
type DeferredServerSummary struct {
	Server    string
	ToolNames []string // capped at promptNameLimit
	Total     int
}

// deferredServerIDs maps the user's deferred MCP server IDs to true.
func deferredServerIDs(user string) map[string]bool {
	counts := map[string]int{}
	for _, t := range tools.GetAll(user) {
		if t.IsEnabled {
			counts[t.MCPServerID]++
		}
	}
	out := map[string]bool{}
	for id, n := range counts {
		if n > promptDeferThreshold {
			out[id] = true
		}
	}
	return out
}

// GetPromptTools returns the enabled tools that should appear in the
// model's tool list. Tool-heavy MCP servers are summarized in the system
// prompt instead (see DeferredServerSummaries).
func GetPromptTools(user string) []*Tool {
	enabled := GetAvailableTools(user)
	deferred := deferredServerIDs(user)
	if len(deferred) == 0 {
		return enabled
	}
	out := make([]*Tool, 0, len(enabled))
	for _, t := range enabled {
		if !deferred[t.MCPServerID] {
			out = append(out, t)
		}
	}
	return out
}

// DeferredServerSummaries lists the deferred servers and their tool names
// for the system prompt, sorted by server name.
func DeferredServerSummaries(user string) []DeferredServerSummary {
	deferred := deferredServerIDs(user)
	if len(deferred) == 0 {
		return nil
	}

	servers := mcps.GetAll(user)
	byID := make(map[string]string, len(servers))
	for _, s := range servers {
		byID[s.ID] = s.Name
	}

	names := make(map[string][]string, len(deferred))
	totals := make(map[string]int, len(deferred))
	for _, t := range tools.GetAll(user) {
		if !t.IsEnabled || !deferred[t.MCPServerID] {
			continue
		}
		totals[t.MCPServerID]++
		if len(names[t.MCPServerID]) < promptNameLimit {
			names[t.MCPServerID] = append(names[t.MCPServerID], t.Name)
		}
	}

	out := make([]DeferredServerSummary, 0, len(deferred))
	for id := range deferred {
		name := byID[id]
		if name == "" {
			name = id
		}
		out = append(out, DeferredServerSummary{
			Server:    name,
			ToolNames: names[id],
			Total:     totals[id],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Server < out[j].Server })
	return out
}

func listToolsTool(args, user string) providers.ToolOutput {
	var params struct {
		Query  string `json:"query"`
		Server string `json:"server"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}
	limit := params.Limit
	if limit <= 0 {
		limit = listToolsDefault
	}
	if limit > listToolsMax {
		limit = listToolsMax
	}
	query := strings.ToLower(strings.TrimSpace(params.Query))
	serverQuery := strings.ToLower(strings.TrimSpace(params.Server))

	deferred := deferredServerIDs(user)
	servers := mcps.GetAll(user)
	serverName := make(map[string]string, len(servers))
	for _, s := range servers {
		serverName[s.ID] = s.Name
	}

	type row struct {
		name, desc, server string
		deferred            bool
	}
	var rows []row
	for _, t := range tools.GetAll(user) {
		if !t.IsEnabled {
			continue
		}
		server := serverName[t.MCPServerID]
		if server == "" {
			server = "built-in"
		}
		if serverQuery != "" && !strings.Contains(strings.ToLower(server), serverQuery) {
			continue
		}
		desc := t.Description
		if query != "" &&
			!strings.Contains(strings.ToLower(t.Name), query) &&
			!strings.Contains(strings.ToLower(desc), query) {
			continue
		}
		rows = append(rows, row{t.Name, desc, server, deferred[t.MCPServerID]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	var b strings.Builder
	fmt.Fprintf(&b, "%d matching tool(s), showing %d", len(rows), min(limit, len(rows)))
	if query != "" {
		fmt.Fprintf(&b, " (query: %q)", params.Query)
	}
	if serverQuery != "" {
		fmt.Fprintf(&b, " (server: %q)", params.Server)
	}
	b.WriteString(".\n")
	shown := 0
	for _, r := range rows {
		if shown >= limit {
			break
		}
		shown++
		b.WriteString("- ")
		b.WriteString(r.name)
		if r.deferred {
			b.WriteString(" [deferred]")
		}
		if r.desc != "" {
			b.WriteString(": ")
			b.WriteString(truncateRunes(strings.TrimSpace(r.desc), listToolsDescRunes))
		}
		fmt.Fprintf(&b, " (server: %s)\n", r.server)
	}
	if rest := len(rows) - shown; rest > 0 {
		fmt.Fprintf(&b, "...and %d more (narrow the query or raise the limit)\n", rest)
	}
	b.WriteString("\nUse get_tool_schema({\"names\": [...]}) for a tool's parameters.")
	return providers.ToolOutput{Content: b.String()}
}

func getToolSchemaTool(args, user string) providers.ToolOutput {
	var params struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}
	if len(params.Names) == 0 {
		return providers.ToolOutput{Content: "Error: 'names' is required."}
	}
	if len(params.Names) > getSchemaMaxNames {
		params.Names = params.Names[:getSchemaMaxNames]
	}

	servers := mcps.GetAll(user)
	serverName := make(map[string]string, len(servers))
	for _, s := range servers {
		serverName[s.ID] = s.Name
	}

	var b strings.Builder
	for _, name := range params.Names {
		name = strings.TrimSpace(name)
		b.WriteString("- ")
		b.WriteString(name)
		tool, err := tools.GetByName(name, user)
		if err != nil {
			b.WriteString(": unknown tool (use list_tools)\n")
			continue
		}
		if !tool.IsEnabled {
			b.WriteString(": disabled\n")
			continue
		}
		server := serverName[tool.MCPServerID]
		if server == "" {
			server = "built-in"
		}
		fmt.Fprintf(&b, " (server: %s)\n", server)
		if tool.Description != "" {
			b.WriteString("  description: ")
			b.WriteString(strings.TrimSpace(tool.Description))
			b.WriteByte('\n')
		}
		schema := tool.InputSchema
		if schema == "" {
			schema = "(no schema)"
		}
		b.WriteString("  schema: ")
		b.WriteString(truncateRunes(schema, getSchemaMaxRunes))
		b.WriteByte('\n')
	}
	return providers.ToolOutput{Content: strings.TrimSpace(b.String())}
}
