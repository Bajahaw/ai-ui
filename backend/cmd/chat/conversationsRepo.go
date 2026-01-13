package chat

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type ConversationRepo interface {
	GetByID(id string, user string) (*Conversation, error)
	Touch(id string, user string) error
	GetAll(user string) []*Conversation
	Save(conversation *Conversation) error
	Update(conversation *Conversation) error
	DeleteByID(id string, user string) error
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

func (repo *ConversationRepository) GetByID(id string, user string) (*Conversation, error) {
	if conv, exists := repo.cache[id]; exists {
		return conv, nil
	}

	query := `SELECT * FROM Conversations WHERE id = ? AND user = ?`
	row := repo.db.QueryRow(query, id, user)

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

func (repo *ConversationRepository) Touch(id string, user string) error {
	query := `UPDATE Conversations SET updated_at = ? WHERE id = ? AND user = ?`
	result, err := repo.db.Exec(query, time.Now().UTC(), id, user)
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

func (repo *ConversationRepository) GetAll(user string) []*Conversation {
	query := `SELECT * FROM Conversations WHERE user = ?`
	var conversations = make([]*Conversation, 0)

	rows, err := repo.db.Query(query, user)
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

func (repo *ConversationRepository) Save(conversation *Conversation) error {
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

func (repo *ConversationRepository) Update(conversation *Conversation) error {
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

func (repo *ConversationRepository) DeleteByID(id string, user string) error {
	query := `DELETE FROM Conversations WHERE id = ? AND user = ?`
	_, err := repo.db.Exec(query, id, user)
	if err != nil {
		return err
	}

	//delete(repo.cache, id)
	return nil
}
