package chat

import (
	"ai-client/cmd/data"
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

func newConversationRepository() *ConversationRepository {
	return &ConversationRepository{
		// todo
		//cache: make(map[string]*Conversation),
	}
}

func (repo *ConversationRepository) getConversation(id string) (*Conversation, error) {
	if conv, exists := repo.cache[id]; exists {
		return conv, nil
	}

	sql := `SELECT * FROM Conversations WHERE id = ?`
	row := data.DB.QueryRow(sql, id)

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
	sql := `UPDATE Conversations SET updated_at = ? WHERE id = ?`
	_, err := data.DB.Exec(sql, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	return nil
}

func (repo *ConversationRepository) getAllConversations() []*Conversation {
	sql := `SELECT * FROM Conversations`
	var conversations = make([]*Conversation, 0)

	rows, err := data.DB.Query(sql)
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
	sql := `INSERT INTO Conversations (id, user, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := data.DB.Exec(sql,
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
	sql := `UPDATE Conversations SET title = ?, updated_at = ? WHERE id = ?`
	_, err := data.DB.Exec(sql,
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
	sql := `DELETE FROM Conversations WHERE id = ?`
	_, err := data.DB.Exec(sql, id)
	if err != nil {
		return err
	}

	//delete(repo.cache, id)
	return nil
}
