package chat

type Message struct {
	ID         int    `json:"id"`
	ConvID     string `json:"convId"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ParentID   int    `json:"parentId,omitempty"`
	Children   []int  `json:"children,omitempty"`
	Attachment string `json:"attachment,omitempty"`
}

func getMessage(id int) (*Message, error) {
	sql := `SELECT id, conv_id, role, content, parent_id, attachment FROM Messages WHERE id = ?`
	row := db.QueryRow(sql, id)

	var msg Message
	err := row.Scan(&msg.ID, &msg.ConvID, &msg.Role, &msg.Content, &msg.ParentID, &msg.Attachment)
	if err != nil {
		return nil, err
	}

	childrenSql := `SELECT id FROM Messages WHERE parent_id = ?`
	rows, err := db.Query(childrenSql, id)
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
	result, err := db.Exec(sql, msg.ConvID, msg.Role, "gpt-3.5-turbo", nil, msg.Attachment, msg.Content)
	if err != nil {
		log.Error("Error inserting message", "err", err)
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Error("Error getting last insert id", "err", err)
		return 0, err
	}

	return int(id), nil
}

func updateMessage(id int, msg Message) error {
	sql := `UPDATE Messages SET content = ? WHERE id = ?`
	_, err := db.Exec(sql, msg.Content, id)
	if err != nil {
		return err
	}
	return nil
}

func getAllConversationMessages(convID string) map[int]*Message {
	messages := make(map[int]*Message)
	sql := `SELECT * FROM Messages WHERE conv_id = ? ORDER BY id ASC`
	rows, err := db.Query(sql, convID)
	if err != nil {
		log.Error("Error querying messages", "err", err)
		return messages
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.ID, &msg.ConvID, &msg.Role, &msg.Content, &msg.ParentID, &msg.Attachment)
		if err != nil {
			log.Error("Error scanning message", "err", err)
			continue
		}
		messages[msg.ID] = &msg
	}

	return messages
}
