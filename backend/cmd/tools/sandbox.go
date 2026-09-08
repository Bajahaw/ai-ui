package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

const (
	sandboxMaxFiles     = 3
	sandboxMaxFileBytes = 15 << 20
	sandboxMaxBodyBytes = 22 << 20
	sandboxMaxLogs      = 8 << 10
	sandboxTimeout      = 90 * time.Second
)

var (
	errSandboxNotPending    = errors.New("no pending sandbox call")
	errSandboxUnauthorized  = errors.New("unauthorized")
	errSandboxTooManyFiles  = errors.New("too many files")
	errSandboxFileTooLarge  = errors.New("file too large")
)

type sandboxClientFile struct {
	Name string
	MIME string
	Data []byte
}

type sandboxClientResult struct {
	OK    bool
	Logs  []string
	Error string
	Files []sandboxClientFile
}

type sandboxPending struct {
	User   string
	Result chan sandboxClientResult
}

var sandboxWait = struct {
	mu      sync.Mutex
	pending map[string]sandboxPending
}{pending: map[string]sandboxPending{}}

func registerSandboxWait(callID, user string) <-chan sandboxClientResult {
	ch := make(chan sandboxClientResult, 1)
	sandboxWait.mu.Lock()
	sandboxWait.pending[callID] = sandboxPending{User: user, Result: ch}
	sandboxWait.mu.Unlock()
	return ch
}

func unregisterSandboxWait(callID, user string) {
	sandboxWait.mu.Lock()
	if p, ok := sandboxWait.pending[callID]; ok && p.User == user {
		delete(sandboxWait.pending, callID)
	}
	sandboxWait.mu.Unlock()
}

func completeSandboxWait(callID, user string, result sandboxClientResult) error {
	sandboxWait.mu.Lock()
	p, ok := sandboxWait.pending[callID]
	if !ok {
		sandboxWait.mu.Unlock()
		return errSandboxNotPending
	}
	if p.User != user {
		sandboxWait.mu.Unlock()
		return errSandboxUnauthorized
	}
	delete(sandboxWait.pending, callID)
	sandboxWait.mu.Unlock()
	p.Result <- result
	return nil
}

func browserSandboxTool(ctx context.Context, callID, args, user string) providers.ToolOutput {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return providers.ToolOutput{Content: "Sandbox was cancelled."}
	}
	if callID == "" {
		return providers.ToolOutput{Content: "Error: missing tool call id."}
	}

	var params struct {
		Code    string   `json:"code"`
		HTML    string   `json:"html"`
		FileIDs []string `json:"file_ids"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}
	if strings.TrimSpace(params.Code) == "" && strings.TrimSpace(params.HTML) == "" {
		return providers.ToolOutput{Content: "Error: 'code' is required."}
	}

	ch := registerSandboxWait(callID, user)
	defer unregisterSandboxWait(callID, user)

	timer := time.NewTimer(sandboxTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return providers.ToolOutput{Content: "Sandbox was cancelled."}
		}
		return providers.ToolOutput{Content: "Sandbox timed out."}
	case <-timer.C:
		return providers.ToolOutput{Content: "Sandbox timed out waiting for the client."}
	case res := <-ch:
		return persistSandboxResult(res, user)
	}
}

func persistSandboxResult(res sandboxClientResult, user string) providers.ToolOutput {
	var b strings.Builder
	if res.OK && res.Error == "" {
		b.WriteString("ok: true\n")
	} else {
		b.WriteString("ok: false\n")
		if res.Error != "" {
			b.WriteString("error: ")
			b.WriteString(res.Error)
			b.WriteByte('\n')
		}
	}

	logs := strings.Join(res.Logs, "\n")
	if utf8.RuneCountInString(logs) > sandboxMaxLogs {
		r := []rune(logs)
		logs = string(r[:sandboxMaxLogs]) + "\n…(logs truncated)"
	}
	if logs != "" {
		b.WriteString("logs:\n")
		b.WriteString(logs)
		b.WriteByte('\n')
	}

	var firstID string
	if len(res.Files) > 0 {
		b.WriteString("files:\n")
	}
	for i, f := range res.Files {
		if i >= sandboxMaxFiles {
			b.WriteString("- (additional files omitted)\n")
			break
		}
		if len(f.Data) > sandboxMaxFileBytes {
			b.WriteString("- skipped ")
			b.WriteString(sanitizeSandboxFilename(f.Name))
			b.WriteString(": file too large\n")
			continue
		}
		name := sanitizeSandboxFilename(f.Name)
		saved, err := saveBinaryFile(f.Data, f.MIME, name, user)
		if err != nil {
			b.WriteString("- failed to save ")
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(err.Error())
			b.WriteByte('\n')
			continue
		}
		if firstID == "" {
			firstID = saved.ID
		}
		b.WriteString("- id: ")
		b.WriteString(saved.ID)
		b.WriteString("\n  name: ")
		b.WriteString(saved.Name)
		b.WriteString("\n  type: ")
		b.WriteString(saved.Type)
		b.WriteString("\n  size: ")
		b.WriteString(fmt.Sprintf("%d", saved.Size))
		b.WriteString("\n  path: ")
		b.WriteString(fs.ResourcePath(saved))
		b.WriteByte('\n')
	}

	return providers.ToolOutput{Content: strings.TrimSpace(b.String()), FileID: firstID}
}

func sanitizeSandboxFilename(name string) string {
	name = strings.TrimSpace(name)
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == ".." {
		return "output.bin"
	}
	return name
}

type sandboxResultRequest struct {
	CallID string              `json:"call_id"`
	OK     bool                `json:"ok"`
	Logs   []string            `json:"logs"`
	Error  string              `json:"error"`
	Files  []sandboxResultFile `json:"files"`
}

type sandboxResultFile struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data string `json:"data"`
}

func submitSandboxResult(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, sandboxMaxBodyBytes)

	var req sandboxResultRequest
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.CallID == "" {
		http.Error(w, "call_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Files) > sandboxMaxFiles {
		http.Error(w, errSandboxTooManyFiles.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	files := make([]sandboxClientFile, 0, len(req.Files))
	for _, f := range req.Files {
		raw, err := base64.StdEncoding.DecodeString(f.Data)
		if err != nil {
			http.Error(w, "invalid file data", http.StatusBadRequest)
			return
		}
		if len(raw) > sandboxMaxFileBytes {
			http.Error(w, errSandboxFileTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		files = append(files, sandboxClientFile{
			Name: f.Name,
			MIME: f.MIME,
			Data: raw,
		})
	}

	err := completeSandboxWait(req.CallID, user, sandboxClientResult{
		OK:    req.OK,
		Logs:  req.Logs,
		Error: req.Error,
		Files: files,
	})
	if errors.Is(err, errSandboxNotPending) {
		http.Error(w, "No pending sandbox call found", http.StatusNotFound)
		return
	}
	if errors.Is(err, errSandboxUnauthorized) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Error completing sandbox", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, nil, http.StatusOK)
}

func EnsureBuiltInTools() {
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT id FROM MCPServers WHERE id LIKE 'default-%'`)
	if err != nil {
		log.Error("Error listing default MCP servers", "err", err)
		return
	}
	defer rows.Close()

	var serverIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		serverIDs = append(serverIDs, id)
	}

	for _, serverID := range serverIDs {
		existing := tools.GetAllByMCPServerID(serverID)
		have := make(map[string]struct{}, len(existing))
		for _, t := range existing {
			have[t.Name] = struct{}{}
		}
		var missing []*Tool
		for _, t := range GetBuiltInTools() {
			if _, ok := have[t.Name]; ok {
				continue
			}
			t.MCPServerID = serverID
			missing = append(missing, t)
		}
		if len(missing) == 0 {
			continue
		}
		if err := tools.UpsertAll(missing); err != nil {
			log.Error("Error seeding built-in tools", "err", err, "server", serverID)
		}
	}
}
