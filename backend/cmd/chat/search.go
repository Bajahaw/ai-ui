package chat

import (
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/data"
)

// ConversationSearchHit is one message match scoped to a conversation the user owns.
type ConversationSearchHit struct {
	ConversationID string    `json:"conversationId"`
	Title          string    `json:"title"`
	MessageID      int       `json:"messageId"`
	Role           string    `json:"role"`
	Snippet        string    `json:"snippet"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// fts5Quote wraps the query in double-quotes so FTS5 treats the entire string
// as a phrase query (same approach as files.fts5Quote).
func fts5Quote(query string) string {
	q := strings.TrimSpace(query)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

// maxSearchQueryRunes caps MATCH input to keep pathological clients from
// forcing huge FTS parses. Normal UI queries are far smaller.
const maxSearchQueryRunes = 200

// searchMessagesFTS runs user-scoped full-text search over message content.
// Returns at most one best hit per conversation (lowest FTS rank), ordered by relevance.
func searchMessagesFTS(user string, query string, limit int) ([]ConversationSearchHit, error) {
	// Defense in depth: never run unscoped search if auth context is missing.
	if strings.TrimSpace(user) == "" {
		return []ConversationSearchHit{}, nil
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return []ConversationSearchHit{}, nil
	}
	if len([]rune(q)) > maxSearchQueryRunes {
		q = string([]rune(q)[:maxSearchQueryRunes])
	}
	if limit <= 0 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}

	// snippet() must be used in the same query level as the FTS MATCH (not inside
	// a subquery). Fetch ranked message hits, then keep the best per conversation.
	// Always join Conversations and filter c.user for auth scope.
	fetchLimit := limit * 4
	if fetchLimit < 40 {
		fetchLimit = 40
	}
	if fetchLimit > 200 {
		fetchLimit = 200
	}

	searchSQL := `
	SELECT
		m.conv_id,
		c.title,
		m.id,
		m.role,
		snippet(MessagesFTS, 0, '[', ']', '…', 24),
		c.updated_at
	FROM MessagesFTS
	JOIN Messages m ON m.rowid = MessagesFTS.rowid
	JOIN Conversations c ON c.id = m.conv_id
	WHERE c.user = ? AND MessagesFTS MATCH ?
	ORDER BY rank
	LIMIT ?
	`

	rows, err := data.DB.Query(searchSQL, user, fts5Quote(q), fetchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := make([]ConversationSearchHit, 0, limit)
	seen := make(map[string]struct{}, limit)
	for rows.Next() {
		var hit ConversationSearchHit
		if err := rows.Scan(
			&hit.ConversationID,
			&hit.Title,
			&hit.MessageID,
			&hit.Role,
			&hit.Snippet,
			&hit.UpdatedAt,
		); err != nil {
			log.Error("Error scanning search hit", "err", err)
			continue
		}
		if _, ok := seen[hit.ConversationID]; ok {
			continue
		}
		seen[hit.ConversationID] = struct{}{}
		if hit.Title == "" {
			hit.Title = "New Chat"
		}
		hits = append(hits, hit)
		if len(hits) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hits, nil
}
