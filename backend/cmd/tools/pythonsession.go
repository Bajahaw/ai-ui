package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session-based sandbox boxes: a persistent on-host directory per session so
// multi-round agent work doesn't lose progress.
//
// Layout under ./data/pywasm/sessions/<uuid>/ (gitignored, app-owned):
//
//	work/      agent files, mounted at /work, persists across rounds
//	mnt/       attachment copies merged per call, mounted at /mnt
//	packages/  extracted wheels, on PYTHONPATH
//	tmp/       main.py overwritten each round
//	out/       deliverables, mounted at /out — collected after each run
//
// Rules:
//   - Sessions are explicit: no session_id → one-shot temp box (old behavior).
//     First call with a session returns "Session-ID:" for the model to reuse.
//   - Sessions are bound to the creating user; every access re-validates.
//   - One round per session at a time (per-session mutex).
//   - Idle TTL + per-user session cap, evicted lazily on access.
//   - Ephemeral across server restarts (registry is in-memory). Documented,
//     not promised otherwise.

var (
	pythonSessionTTL      = 30 * time.Minute
	pythonSessionMax      = 5
	pythonSessionMaxMount = 20
	pythonSessionMaxBytes = int64(50 << 20) // attachments merged per call
	pythonOutMaxFiles     = 10
	pythonOutMaxFileBytes = int64(25 << 20)
	pythonOutMaxTotal     = int64(100 << 20)
	pythonBoxMaxDisk      = int64(200 << 20)

	reSessionID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

	pythonSessionsMu sync.Mutex
	pythonSessions   = make(map[string]*pythonSession)
)

type deliveredFile struct {
	size    int64
	modTime int64
	fileID  string
}

type pythonSession struct {
	id        string
	user      string
	root      string
	created   time.Time
	lastUsed  time.Time
	delivered map[string]deliveredFile
	mu        sync.Mutex
}

func pythonSessionsDir() string {
	return filepath.Join(pythonEngineDir(), "sessions")
}

// getPythonSession resolves sessionID for user, creating a fresh box when
// empty, unknown, expired, or owned by someone else. expired=true tells the
// caller the requested box is gone (output should say so; work continues in
// the fresh box rather than erroring).
func getPythonSession(user, sessionID string) (*pythonSession, bool, error) {
	pythonSessionsMu.Lock()
	defer pythonSessionsMu.Unlock()

	now := time.Now()
	// Lazy eviction on every access.
	for id, s := range pythonSessions {
		if now.Sub(s.lastUsed) > pythonSessionTTL {
			_ = os.RemoveAll(s.root)
			delete(pythonSessions, id)
		}
	}

	if sessionID != "" {
		if !reSessionID.MatchString(sessionID) {
			return nil, false, fmt.Errorf("invalid session_id %q", sessionID)
		}
		if s, ok := pythonSessions[sessionID]; ok && s.user == user {
			s.lastUsed = now
			return s, false, nil
		}
		// Unknown/expired/foreign: fresh box, caller reports it.
		s, err := newPythonSessionLocked(user)
		if err != nil {
			return nil, false, err
		}
		return s, true, nil
	}
	s, err := newPythonSessionLocked(user)
	if err != nil {
		return nil, false, err
	}
	return s, false, nil
}

func newPythonSessionLocked(user string) (*pythonSession, error) {
	// Cap boxes per user: evict this user's oldest idle first.
	var owned []*pythonSession
	for _, s := range pythonSessions {
		if s.user == user {
			owned = append(owned, s)
		}
	}
	if len(owned) >= pythonSessionMax {
		sort.Slice(owned, func(i, j int) bool { return owned[i].lastUsed.Before(owned[j].lastUsed) })
		victim := owned[0]
		_ = os.RemoveAll(victim.root)
		delete(pythonSessions, victim.id)
	}
	id := uuid.New().String()
	root := filepath.Join(pythonSessionsDir(), id)
	for _, d := range []string{"work", "mnt", "packages", "tmp", "out", filepath.Join("usr", "local", "lib")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	s := &pythonSession{
		id: id, user: user, root: root,
		created: time.Now(), lastUsed: time.Now(),
		delivered: make(map[string]deliveredFile),
	}
	pythonSessions[id] = s
	return s, nil
}

func (s *pythonSession) diskUsage() int64 {
	var total int64
	_ = filepath.Walk(s.root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// mountAttachmentFiles copies user-owned attachments into the session mnt dir
// (copies, never live mounts: the guest cannot touch the originals).
// Returns guest paths merged this call.
func mountAttachmentFiles(sess *pythonSession, user string, ids []string) ([]string, error) {
	if len(ids) > pythonSessionMaxMount {
		return nil, fmt.Errorf("at most %d mount_files per call (%d given)", pythonSessionMaxMount, len(ids))
	}
	docs, err := files.GetByIDs(ids, user)
	if err != nil {
		return nil, fmt.Errorf("resolving attachments: %w", err)
	}
	if len(docs) != len(ids) {
		return nil, errors.New("one or more file_ids not found or not yours")
	}
	var total int64
	var merged []string
	for _, doc := range docs {
		data, err := os.ReadFile(doc.Path)
		if err != nil {
			return nil, fmt.Errorf("reading attachment %q: %w", doc.Name, err)
		}
		total += int64(len(data))
		if total > pythonSessionMaxBytes {
			return nil, fmt.Errorf("attachments exceed %d bytes per call", pythonSessionMaxBytes)
		}
		name := dedupMountName(sess, doc.Name)
		if err := os.WriteFile(filepath.Join(sess.root, "mnt", name), data, 0o644); err != nil {
			return nil, err
		}
		merged = append(merged, "/mnt/"+name)
	}
	return merged, nil
}

func dedupMountName(sess *pythonSession, name string) string {
	name = filepath.Base(name)
	if name == "" || name == "." || name == "/" {
		name = "attachment"
	}
	candidate := name
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(sess.root, "mnt", candidate)); os.IsNotExist(err) {
			return candidate
		}
		ext := filepath.Ext(name)
		candidate = strings.TrimSuffix(name, ext) + " (" + strconv.Itoa(i) + ")" + ext
	}
}

// collectOutFiles delivers new/changed files under /out as chat attachments.
// First file becomes fileID (existing ToolOutput convention, same as the
// image/PDF tools); the rest are saved and listed in notes. Already-delivered
// files (same path+size+mtime) are skipped so repeat rounds don't re-attach.
func collectOutFiles(sess *pythonSession, user string) (fileID, notes string) {
	outDir := filepath.Join(sess.root, "out")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return "", ""
	}
	var fresh []string
	var skipped int
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		rel := e.Name()
		if !validOutName(rel) {
			skipped++
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if d, ok := sess.delivered[rel]; ok && d.size == info.Size() && d.modTime == info.ModTime().UnixNano() {
			continue
		}
		if len(fresh) >= pythonOutMaxFiles || total+info.Size() > pythonOutMaxTotal {
			skipped++
			continue
		}
		if info.Size() > pythonOutMaxFileBytes {
			skipped++
			continue
		}
		fresh = append(fresh, rel)
		total += info.Size()
	}
	var b strings.Builder
	for _, rel := range fresh {
		data, err := os.ReadFile(filepath.Join(outDir, rel))
		if err != nil {
			fmt.Fprintf(&b, "Could not read /out/%s: %v\n", rel, err)
			continue
		}
		f, err := saveBinaryFile(data, "", rel, user)
		if err != nil {
			fmt.Fprintf(&b, "Could not save /out/%s: %v\n", rel, err)
			continue
		}
		info, _ := os.Stat(filepath.Join(outDir, rel))
		sess.delivered[rel] = deliveredFile{size: info.Size(), modTime: info.ModTime().UnixNano(), fileID: f.ID}
		if fileID == "" {
			fileID = f.ID
		}
		fmt.Fprintf(&b, "Saved file: %s (id: %s, %d bytes)\n", rel, f.ID, len(data))
	}
	if skipped > 0 {
		fmt.Fprintf(&b, "(%d file(s) in /out skipped: over limit, oversize, or invalid name)\n", skipped)
	}
	return fileID, b.String()
}

func validOutName(name string) bool {
	if name == "" || name == "." || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return false
	}
	return true
}

// workspaceListing prints session files (names + sizes) for the model.
func workspaceListing(sess *pythonSession) string {
	var b strings.Builder
	for _, dir := range []string{"work", "mnt", "out"} {
		entries, err := os.ReadDir(filepath.Join(sess.root, dir))
		if err != nil {
			continue
		}
		var shown int
		for _, e := range entries {
			if shown >= 50 {
				b.WriteString("  ...\n")
				break
			}
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			fmt.Fprintf(&b, "  /%s/%s (%d bytes)\n", dir, e.Name(), info.Size())
			shown++
		}
		if shown == 0 {
			fmt.Fprintf(&b, "  /%s/ (empty)\n", dir)
		}
	}
	return b.String()
}
