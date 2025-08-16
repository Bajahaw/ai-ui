package chat

import "errors"

type ConversationRepo interface {
	GetConversation(id string) (*Conversation, error)
	GetAllConversations() ([]*Conversation, error)
	AddConversation(conversation *Conversation) error
	UpdateConversation(conversation *Conversation) error
	DeleteConversation(id string) error
}

type InMemoryConversationRepo struct {
	conversations map[string]*Conversation
}

func NewInMemoryConversationRepo() *InMemoryConversationRepo {
	return &InMemoryConversationRepo{
		conversations: make(map[string]*Conversation),
	}
}

func (repo *InMemoryConversationRepo) GetConversation(id string) (*Conversation, error) {
	if conversation, exists := repo.conversations[id]; exists {
		return conversation, nil
	}
	return nil, errors.New("conversation not found")
}

func (repo *InMemoryConversationRepo) GetAllConversations() ([]*Conversation, error) {
	allConversations := make([]*Conversation, 0, len(repo.conversations))
	for _, conversation := range repo.conversations {
		allConversations = append(allConversations, conversation)
	}
	return allConversations, nil
}

func (repo *InMemoryConversationRepo) AddConversation(conversation *Conversation) error {
	if _, exists := repo.conversations[conversation.ID]; exists {
		return errors.New("conversation already exists")
	}
	repo.conversations[conversation.ID] = conversation
	return nil
}

func (repo *InMemoryConversationRepo) UpdateConversation(conversation *Conversation) error {
	if _, exists := repo.conversations[conversation.ID]; !exists {
		return errors.New("conversation not found")
	}
	repo.conversations[conversation.ID] = conversation
	return nil
}

func (repo *InMemoryConversationRepo) DeleteConversation(id string) error {
	if _, exists := repo.conversations[id]; !exists {
		return errors.New("conversation not found")
	}
	delete(repo.conversations, id)
	return nil
}
