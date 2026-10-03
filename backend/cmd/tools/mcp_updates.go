package tools

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/utils"
)

const (
	// Checks run on app load, so the interval keeps reloads from refetching
	// and makes a check land when provider prompt caches are cold anyway.
	mcpUpdateCheckInterval = 6 * time.Hour
	mcpUpdateCheckTimeout  = 15 * time.Second
	// How long a chat request waits for an in-flight check before it builds
	// the tool list, so a check finishing mid-conversation can't swap tools
	// between the first and second message.
	mcpUpdateWaitTimeout = 5 * time.Second
)

type MCPUpdateCheckResponse struct {
	Updated []string `json:"updated"`
	Pending []string `json:"pending"`
}

type mcpUpdateRun struct {
	done   chan struct{}
	result MCPUpdateCheckResponse
}

var mcpUpdateRuns = struct {
	mu   sync.Mutex
	runs map[string]*mcpUpdateRun
}{runs: make(map[string]*mcpUpdateRun)}

func checkMCPUpdates(w http.ResponseWriter, r *http.Request) {
	utils.RespondWithJSON(w, CheckMCPUpdates(utils.ExtractContextUser(r)), http.StatusOK)
}

// CheckMCPUpdates checks the user's servers that are due. Servers with
// auto-update on get changes applied; others are only flagged as pending.
// Concurrent calls for the same user share one run.
func CheckMCPUpdates(user string) MCPUpdateCheckResponse {
	mcpUpdateRuns.mu.Lock()
	if run, ok := mcpUpdateRuns.runs[user]; ok {
		mcpUpdateRuns.mu.Unlock()
		<-run.done
		return run.result
	}
	run := &mcpUpdateRun{done: make(chan struct{})}
	mcpUpdateRuns.runs[user] = run
	mcpUpdateRuns.mu.Unlock()

	run.result = checkDueMCPServers(user, time.Now())

	mcpUpdateRuns.mu.Lock()
	delete(mcpUpdateRuns.runs, user)
	mcpUpdateRuns.mu.Unlock()
	close(run.done)
	return run.result
}

// WaitForMCPUpdateCheck blocks briefly while an update check runs for user.
func WaitForMCPUpdateCheck(user string) {
	mcpUpdateRuns.mu.Lock()
	run, ok := mcpUpdateRuns.runs[user]
	mcpUpdateRuns.mu.Unlock()
	if !ok {
		return
	}
	select {
	case <-run.done:
	case <-time.After(mcpUpdateWaitTimeout):
	}
}

func checkDueMCPServers(user string, now time.Time) MCPUpdateCheckResponse {
	resp := MCPUpdateCheckResponse{Updated: []string{}, Pending: []string{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, server := range mcps.GetAll(user) {
		// Built-ins change only on deploy and need no network, so they are
		// compared on every check and always applied.
		if isBuiltInServer(server.ID) {
			if updated, err := syncBuiltInServer(server); err != nil {
				log.Error("Built-in tools sync failed", "server", server.ID, "err", err)
			} else if updated {
				mu.Lock()
				resp.Updated = append(resp.Updated, server.ID)
				mu.Unlock()
			}
			continue
		}
		if !mcpUpdateCheckDue(server, now) {
			continue
		}
		wg.Add(1)
		go func(server *MCPServer) {
			defer wg.Done()
			updated, pending := checkMCPServerUpdate(server, now)
			mu.Lock()
			defer mu.Unlock()
			if updated {
				resp.Updated = append(resp.Updated, server.ID)
			} else if pending {
				resp.Pending = append(resp.Pending, server.ID)
			}
		}(server)
	}
	wg.Wait()
	return resp
}

func mcpUpdateCheckDue(server *MCPServer, now time.Time) bool {
	if server.AuthType == AuthTypeOAuth2 && !server.OAuth.connected() {
		return false
	}
	return now.Sub(time.Unix(server.LastCheckedAt, 0)) >= mcpUpdateCheckInterval
}

type mcpCatalog struct {
	tools []*Tool
	info  MCPServerInfo
	err   error
}

func checkMCPServerUpdate(server *MCPServer, now time.Time) (updated, pending bool) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpUpdateCheckTimeout)
	defer cancel()

	// Connecting has its own longer timeout; stop waiting at ours and let a
	// slow connect finish in the background.
	ch := make(chan mcpCatalog, 1)
	go func() {
		tools, info, err := fetchMCPCatalog(ctx, *server)
		ch <- mcpCatalog{tools, info, err}
	}()
	var fresh mcpCatalog
	select {
	case fresh = <-ch:
	case <-ctx.Done():
		fresh.err = ctx.Err()
	}

	if fresh.err != nil {
		log.Warn("MCP update check failed", "server", server.ID, "err", fresh.err)
		// Retry at the next interval, not on every app load.
		recordMCPCheck(server, server.UpdatePending, now)
		return false, false
	}

	if !mcpCatalogChanged(server, fresh.tools, fresh.info) {
		recordMCPCheck(server, false, now)
		return false, false
	}
	if !server.AutoUpdate {
		recordMCPCheck(server, true, now)
		return false, true
	}
	if err := storeCatalog(server.ID, server.User, fresh.tools, fresh.info); err != nil {
		log.Error("MCP auto-update failed", "server", server.ID, "err", err)
		recordMCPCheck(server, true, now)
		return false, true
	}
	return true, false
}

func recordMCPCheck(server *MCPServer, pending bool, now time.Time) {
	if err := mcps.UpdateSyncState(server.ID, server.User, pending, now.Unix()); err != nil {
		log.Error("Failed to record MCP update check", "server", server.ID, "err", err)
	}
}

// mcpCatalogChanged compares a fresh catalog with the stored one, ignoring
// tool order.
func mcpCatalogChanged(server *MCPServer, fresh []*Tool, info MCPServerInfo) bool {
	if server.Info != info || len(server.Tools) != len(fresh) {
		return true
	}
	stored := make(map[string]*Tool, len(server.Tools))
	for _, t := range server.Tools {
		stored[t.Name] = t
	}
	for _, t := range fresh {
		s, ok := stored[t.Name]
		if !ok || s.Description != t.Description || s.InputSchema != t.InputSchema {
			return true
		}
	}
	return false
}
