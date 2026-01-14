package chat

import (
	"ai-client/cmd/data"
	fs "ai-client/cmd/files"
	"ai-client/cmd/tools"
)

type Message struct {
	ID          int              `json:"id"`
	ConvID      string           `json:"convId"`
	Role        string           `json:"role"`
	Model       string           `json:"model,omitempty"`
	Content     string           `json:"content"`
	Reasoning   string           `json:"reasoning,omitempty"`
	ParentID    int              `json:"parentId,omitempty"`
	Children    []int            `json:"children"`
	Attachments []Attachment     `json:"attachments,omitempty"`
	Error       string           `json:"error,omitempty"`
	Tools       []tools.ToolCall `json:"tools,omitempty"`
}

type Attachment struct {
	ID        string  `json:"id"`
	MessageID int     `json:"messageId"`
	File      fs.File `json:"file"`
}

func getMessage(id int) (*Message, error) {
	sql := `
	SELECT id, conv_id, role, content, reasoning, parent_id, error 
	FROM Messages 
	WHERE id = ?
	`
	row := data.DB.QueryRow(sql, id)

	var msg = Message{
		Children: make([]int, 0),
	}
	err := row.Scan(
		&msg.ID, &msg.ConvID,
		&msg.Role,
		&msg.Content,
		&msg.Reasoning,
		&msg.ParentID,
		&msg.Error)
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

	// Fetch attachments
	msg.Attachments = getMessageAttachments(id)

	// Fetch tool calls
	toolCalls := toolCalls.GetAllByMessageID(id)
	msg.Tools = make([]tools.ToolCall, 0)
	for _, t := range toolCalls {
		msg.Tools = append(msg.Tools, *t)
	}

	return &msg, nil
}

func saveMessage(msg Message) (int, error) {
	sql := `
	INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error) 
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := data.DB.Exec(sql,
		msg.ConvID,
		msg.Role,
		msg.Model,
		msg.ParentID,
		msg.Content,
		msg.Reasoning,
		msg.Error,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	intId := int(id)
	err = saveMessageAttachments(intId, msg.Attachments)
	if err != nil {
		return 0, err
	}

	return intId, nil
}

func saveMessageAttachments(id int, attachments []Attachment) error {
	attSql := `INSERT INTO Attachments (id, message_id, file_id) VALUES (?, ?, ?)`
	for _, att := range attachments {
		_, err := data.DB.Exec(attSql,
			att.ID,
			id,
			att.File.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateMessage(id int, msg Message) (*Message, error) {
	sql := `
	UPDATE Messages SET content = ?, reasoning = ?, error = ?
	WHERE id = ?
	RETURNING id, conv_id, role, model, content, reasoning, parent_id, error
	`
	row := data.DB.QueryRow(sql, msg.Content, msg.Reasoning, msg.Error, id)

	var updatedMsg Message
	err := row.Scan(
		&updatedMsg.ID,
		&updatedMsg.ConvID,
		&updatedMsg.Role,
		&updatedMsg.Model,
		&updatedMsg.Content,
		&updatedMsg.Reasoning,
		&updatedMsg.ParentID,
		&updatedMsg.Error,
	)

	if err != nil {
		return nil, err
	}

	updatedMsg.Children = make([]int, 0)
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
		updatedMsg.Children = append(updatedMsg.Children, childID)
	}

	// Fetch attachments
	updatedMsg.Attachments = getMessageAttachments(id)

	// Fetch tool calls
	toolCalls := toolCalls.GetAllByMessageID(id)
	updatedMsg.Tools = make([]tools.ToolCall, 0)
	for _, tool := range toolCalls {
		updatedMsg.Tools = append(updatedMsg.Tools, *tool)
	}

	return &updatedMsg, nil
}

func getAllConversationMessages(convID string, user string) map[int]*Message {
	messages := make(map[int]*Message)
	sql := ` 
	SELECT m.id, m.conv_id, m.role, m.model, m.content, m.reasoning, m.parent_id, m.error
	FROM Messages m 
	INNER JOIN Conversations c ON m.conv_id = c.id
	WHERE m.conv_id = ? AND c.user = ? 
	`
	rows, err := data.DB.Query(sql, convID, user)
	if err != nil {
		log.Error("Error querying messages", "err", err)
		return messages
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		err := rows.Scan(
			&msg.ID,
			&msg.ConvID,
			&msg.Role,
			&msg.Model,
			&msg.Content,
			&msg.Reasoning,
			&msg.ParentID,
			&msg.Error,
		)
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

	// Fetch attachments for all messages in the conversation
	attachments := getAllConversationAttachments(convID)
	for msgID, atts := range attachments {
		if msg, exists := messages[msgID]; exists {
			msg.Attachments = atts
		}
	}

	toolCalls := toolCalls.GetAllByConvID(convID)
	log.Debug("Retrieved tool calls for conversation", "convID", convID, "tools", toolCalls)
	for _, tool := range toolCalls {
		if msg, exists := messages[tool.MessageID]; exists {
			msg.Tools = append(msg.Tools, *tool)
		}
	}

	return messages
}

func getMessageAttachments(messageID int) []Attachment {
	attachmentsSql := `
	SELECT a.id, a.message_id, f.id, f.name, f.type, f.size, f.path, f.url, f.content, f.created_at
	FROM Attachments a
	JOIN Files f ON a.file_id = f.id
	WHERE a.message_id = ?
	`
	attRows, err := data.DB.Query(attachmentsSql, messageID)
	if err != nil {
		log.Error("Error querying attachments", "err", err)
		return nil
	}
	defer attRows.Close()

	attachments := make([]Attachment, 0)
	for attRows.Next() {
		var att Attachment
		var file fs.File
		if err := attRows.Scan(
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
			return nil
		}
		att.File = file
		attachments = append(attachments, att)
	}

	return attachments
}

func getAllConversationAttachments(convID string) map[int][]Attachment {
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
		var file fs.File
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
