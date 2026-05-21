package files

import (
	"database/sql"

	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

type File struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Path       string `json:"path"`
	URL        string `json:"url"`
	Content    string `json:"content"`
	User       string `json:"user,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UploadedAt string `json:"uploadedAt"`
}

type FilePage struct {
	ID         string `json:"id"`
	FileID     string `json:"file_id"`
	PageNumber int    `json:"page_number"`
	Content    string `json:"content"`
}

type Attachment struct {
	ID        string `json:"id"`
	MessageID int    `json:"messageId"`
	File      File   `json:"file"`
}

type Repository interface {
	GetAll(user string) ([]File, error)
	GetByIDs(fileIDs []string, user string) ([]File, error)
	Save(file File) error
	SavePages(pages []FilePage) error
	GetPage(fileID string, pageNumber int) (FilePage, error)
	GetPagesRange(fileID string, startPage int, endPage int) ([]FilePage, error)
	SearchPages(fileID string, query string, limit int) ([]FilePage, error)
	UpdateContent(id string, user string, content string) error
	DeleteByID(id string, user string) error
	GetAllConversationAttachments(convID string) map[int][]Attachment
}

type RepositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (r *RepositoryImpl) GetAll(user string) ([]File, error) {
	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at, uploaded_at
	FROM Files
	WHERE user = ?
	`

	rows, err := r.db.Query(fileSql, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
			&file.UploadedAt,
		); err != nil {
			continue // mimic original behavior of skipping errors?
		}
		files = append(files, file)
	}
	return files, nil
}

func (r *RepositoryImpl) GetByIDs(fileIDs []string, user string) ([]File, error) {
	if len(fileIDs) == 0 {
		return []File{}, nil
	}

	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at, uploaded_at
	FROM Files
	WHERE id IN (` + utils.SqlPlaceholders(len(fileIDs)) + `) AND user = ?
	`

	args := make([]any, len(fileIDs)+1)
	for i, id := range fileIDs {
		args[i] = id
	}
	args[len(fileIDs)] = user

	rows, err := r.db.Query(fileSql, args...)
	if err != nil {
		return []File{}, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
			&file.UploadedAt,
		); err != nil {
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

func (r *RepositoryImpl) Save(file File) error {
	attSql := `INSERT INTO Files (id, name, type, size, path, url, content, user, created_at, uploaded_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(attSql,
		file.ID,
		file.Name,
		file.Type,
		file.Size,
		file.Path,
		file.URL,
		file.Content,
		file.User,
		file.CreatedAt,
		file.UploadedAt,
	)
	return err
}

func (r *RepositoryImpl) SavePages(pages []FilePage) error {
	if len(pages) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pageSql := `INSERT INTO FilePages (id, file_id, page_number, content) VALUES (?, ?, ?, ?)`
	stmt, err := tx.Prepare(pageSql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, page := range pages {
		_, err := stmt.Exec(page.ID, page.FileID, page.PageNumber, page.Content)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *RepositoryImpl) GetPage(fileID string, pageNumber int) (FilePage, error) {
	var page FilePage
	query := `SELECT id, file_id, page_number, content FROM FilePages WHERE file_id = ? AND page_number = ? LIMIT 1`
	err := r.db.QueryRow(query, fileID, pageNumber).Scan(&page.ID, &page.FileID, &page.PageNumber, &page.Content)
	if err != nil {
		return page, err
	}
	return page, nil
}

func (r *RepositoryImpl) GetPagesRange(fileID string, startPage int, endPage int) ([]FilePage, error) {
	var pages []FilePage
	query := `SELECT id, file_id, page_number, content FROM FilePages WHERE file_id = ? AND page_number >= ? AND page_number <= ? ORDER BY page_number ASC`
	rows, err := r.db.Query(query, fileID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var page FilePage
		err := rows.Scan(&page.ID, &page.FileID, &page.PageNumber, &page.Content)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func (r *RepositoryImpl) SearchPages(fileID string, query string, limit int) ([]FilePage, error) {
	// Use fts5 for fast matching on content
	// We extract ID from the FTS search using content_rowid logic or simply join.
	// Since we defined FTS table syncing with triggers (rowid), we can join.
	searchSql := `
	SELECT p.id, p.file_id, p.page_number, snippet(FilePagesFTS, 0, '[', ']', '...', 256) as content
	FROM FilePagesFTS fts
	JOIN FilePages p ON p.rowid = fts.rowid
	WHERE p.file_id = ? AND FilePagesFTS MATCH ?
	ORDER BY rank
	LIMIT ?
	`
	rows, err := r.db.Query(searchSql, fileID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []FilePage
	for rows.Next() {
		var page FilePage
		if err := rows.Scan(&page.ID, &page.FileID, &page.PageNumber, &page.Content); err != nil {
			continue
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func (r *RepositoryImpl) UpdateContent(id string, user string, content string) error {
	updateSql := `UPDATE Files SET content = ? WHERE id = ? AND user = ?`
	_, err := r.db.Exec(updateSql, content, id, user)
	return err
}

func (r *RepositoryImpl) DeleteByID(id string, user string) error {
	deleteSql := `DELETE FROM Files WHERE id = ? AND user = ?`
	_, err := r.db.Exec(deleteSql, id, user)
	return err
}

func (r *RepositoryImpl) GetAllConversationAttachments(convID string) map[int][]Attachment {
	attachments := make(map[int][]Attachment)
	sql := `
	SELECT a.id, a.message_id, f.id, f.name, f.type, f.size, f.path, f.url, f.content, f.created_at
	FROM Attachments a
	JOIN Messages m ON a.message_id = m.id
	JOIN Files f ON a.file_id = f.id
	WHERE m.conv_id = ?
	`
	rows, err := data.DB.Query(sql, convID)
	if err != nil {
		log.Error("Error querying conversation attachments", "err", err)
		return attachments
	}
	defer rows.Close()

	for rows.Next() {
		var att Attachment
		var file File
		if err := rows.Scan(
			&att.ID,
			&att.MessageID,
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
		); err != nil {
			log.Error("Error scanning attachment", "err", err)
			continue
		}
		att.File = file
		attachments[att.MessageID] = append(attachments[att.MessageID], att)
	}

	return attachments
}
