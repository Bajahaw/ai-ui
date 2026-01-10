package chat

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type ConversationRepo interface {
	getConversation(id string) (*Conversation, error)
	touchConversation(id string) error
	getAllConversations() []*Conversation
	saveConversation(conversation *Conversation) error
	updateConversation(conversation *Conversation) error
	deleteConversation(id string) error
}

type ConversationRepository struct {
	db    *sql.DB
	cache map[string]*Conversation
}

func newConversation(userId string) *Conversation {
	return &Conversation{
		ID:        uuid.New().String(),
		UserID:    userId,
		Title:     "",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func newConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{
		db: db,
		// todo
		//cache: make(map[string]*Conversation),
	}
}

func (repo *ConversationRepository) getConversation(id string) (*Conversation, error) {
	if conv, exists := repo.cache[id]; exists {
		return conv, nil
	}

	query := `SELECT * FROM Conversations WHERE id = ?`
	row := repo.db.QueryRow(query, id)

	var conv Conversation
	err := row.Scan(
		&conv.ID,
		&conv.UserID,
		&conv.Title,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if err == nil {
		//repo.cache[id] = &conv
		return &conv, nil
	}

	return nil, errors.New("conversation not found")
}

func (repo *ConversationRepository) touchConversation(id string) error {
	query := `UPDATE Conversations SET updated_at = ? WHERE id = ?`
	result, err := repo.db.Exec(query, time.Now().UTC(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("conversation not found")
	}

	return nil
}

func (repo *ConversationRepository) getAllConversations() []*Conversation {
	query := `SELECT * FROM Conversations`
	var conversations = make([]*Conversation, 0)

	rows, err := repo.db.Query(query)
	if err != nil {
		log.Error("Error querying conversations", "err", err)
		return conversations
	}
	defer rows.Close()

	for rows.Next() {
		var conv Conversation
		err := rows.Scan(
			&conv.ID,
			&conv.UserID,
			&conv.Title,
			&conv.CreatedAt,
			&conv.UpdatedAt,
		)
		if err != nil {
			return conversations
		}
		conversations = append(conversations, &conv)
	}

	return conversations
}

func (repo *ConversationRepository) saveConversation(conversation *Conversation) error {
	query := `INSERT INTO Conversations (id, user, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(query,
		conversation.ID,
		conversation.UserID,
		conversation.Title,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)
	if err != nil {
		return err
	}

	//repo.cache[conversation.ID] = conversation
	return nil
}

func (repo *ConversationRepository) updateConversation(conversation *Conversation) error {
	query := `UPDATE Conversations SET title = ?, updated_at = ? WHERE id = ?`
	_, err := repo.db.Exec(query,
		conversation.Title,
		conversation.UpdatedAt,
		conversation.ID,
	)
	if err != nil {
		return err
	}

	//repo.cache[conversation.ID] = conversation
	return nil
}

func (repo *ConversationRepository) deleteConversation(id string) error {
	query := `DELETE FROM Conversations WHERE id = ?`
	_, err := repo.db.Exec(query, id)
	if err != nil {
		return err
	}

	//delete(repo.cache, id)
	return nil
}
