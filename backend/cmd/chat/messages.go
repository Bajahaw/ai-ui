package chat

import "ai-client/cmd/data"

type Message struct {
	ID         int    `json:"id"`
	ConvID     string `json:"convId"`
	Role       string `json:"role"`
	Model      string `json:"model,omitempty"`
	Content    string `json:"content"`
	ParentID   int    `json:"parentId,omitempty"`
	Children   []int  `json:"children"`
	Attachment string `json:"attachment,omitempty"`
}

func getMessage(id int) (*Message, error) {
	sql := `SELECT id, conv_id, role, content, parent_id, attachment FROM Messages WHERE id = ?`
	row := data.DB.QueryRow(sql, id)

	var msg = Message{
		Children: make([]int, 0),
	}
	err := row.Scan(&msg.ID, &msg.ConvID, &msg.Role, &msg.Content, &msg.ParentID, &msg.Attachment)
	if err != nil {
		return nil, err
	}

	childrenSql := `SELECT id FROM Messages WHERE parent_id = ?`
	rows, err := data.DB.Query(childrenSql, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var childID int
		if err := rows.Scan(&childID); err != nil {
			return nil, err
		}
		msg.Children = append(msg.Children, childID)
	}

	return &msg, nil
}

func saveMessage(msg Message) (int, error) {
	sql := `INSERT INTO Messages (conv_id, role, model, parent_id, attachment, content) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := data.DB.Exec(sql, msg.ConvID, msg.Role, msg.Model, msg.ParentID, msg.Attachment, msg.Content)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func updateMessage(id int, msg Message) (*Message, error) {
	sql := `UPDATE Messages SET content = ? WHERE id = ? RETURNING id, conv_id, role, model, content, parent_id, attachment`
	row := data.DB.QueryRow(sql, msg.Content, id)

	var updatedMsg Message
	err := row.Scan(
		&updatedMsg.ID,
		&updatedMsg.ConvID,
		&updatedMsg.Role,
		&updatedMsg.Model,
		&updatedMsg.Content,
		&updatedMsg.ParentID,
		&updatedMsg.Attachment,
	)

	if err != nil {
		return nil, err
	}

	return &updatedMsg, nil
}

func getAllConversationMessages(convID string) map[int]*Message {
	messages := make(map[int]*Message)
	sql := `SELECT id, conv_id, role, model, content, parent_id, attachment FROM Messages WHERE conv_id = ?`
	rows, err := data.DB.Query(sql, convID)
	if err != nil {
		log.Error("Error querying messages", "err", err)
		return messages
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.ID, &msg.ConvID, &msg.Role, &msg.Model, &msg.Content, &msg.ParentID, &msg.Attachment)
		if err != nil {
			log.Error("Error scanning message", "err", err)
			continue
		}
		messages[msg.ID] = &msg
	}

	for _, msg := range messages {
		if msg.Children == nil {
			msg.Children = make([]int, 0)
		}
		if msg.ParentID != 0 {
			if parent, exists := messages[msg.ParentID]; exists {
				if parent.Children == nil {
					parent.Children = make([]int, 0)
				}
				parent.Children = append(parent.Children, msg.ID)
			}
		}
	}

	return messages
}
