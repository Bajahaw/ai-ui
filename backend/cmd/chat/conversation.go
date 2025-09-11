package chat

import (
	"errors"
	"sync"
)

type Message struct {
	ID         int    `json:"id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ParentID   int    `json:"parentId,omitempty"`
	Children   []int  `json:"children"`
	Attachment string `json:"attachment,omitempty"`
}

type Conversation struct {
	ID       string           `json:"id"`
	Title    string           `json:"title,omitempty"`
	Messages map[int]*Message `json:"messages"`
	Root     []int            `json:"root"`

	mu     sync.Mutex
	NextID int
}

func NewConversation(id string) *Conversation {
	return &Conversation{
		ID:       id,
		Title:    "",
		Messages: make(map[int]*Message),
		Root:     []int{},
		NextID:   0,
	}
}

func (c *Conversation) GetMessage(id int) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if msg, ok := c.Messages[id]; ok {
		return msg, nil
	}
	return nil, errors.New("message not found")
}

// AppendMessage always assigns a new unique ID and returns it.
func (c *Conversation) AppendMessage(msg Message) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.NextID <= 0 && len(c.Messages) > 0 {
		for id := range c.Messages {
			if id > c.NextID {
				c.NextID = id
			}
		}
	}
	c.NextID++
	newID := c.NextID

	stored := &Message{
		ID:       newID,
		Role:     msg.Role,
		Content:  msg.Content,
		ParentID: msg.ParentID,
		Children: []int{},
	}

	if parent, ok := c.Messages[msg.ParentID]; ok {
		parent.Children = append(parent.Children, newID)
	} else {
		// active referenced a missing message -> treat as root
		c.Root = append(c.Root, newID)
	}

	c.Messages[newID] = stored

	return newID
}

func (c *Conversation) UpdateMessage(id int, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Messages[id]; !exists {
		return errors.New("message not found")
	}

	stored := c.Messages[id]
	stored.Role = msg.Role
	stored.Content = msg.Content
	// stored.ParentID = msg.ParentID
	// stored.Children = msg.Children

	return nil
}
