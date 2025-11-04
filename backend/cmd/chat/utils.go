package chat

import (
	"ai-client/cmd/provider"
)

// Helper
func buildContext(convID string, start int) []provider.SimpleMessage {
	var convMessages = getAllConversationMessages(convID) // todo: cache or something
	var path []int
	var current = start
	log.Debug("Current message ID", "id", current)
	for {
		leaf, ok := convMessages[current]
		if !ok {
			break
		}
		path = append(path, current)
		current = leaf.ParentID
	}

	systemPrompt, _ := getSetting("systemPrompt")

	var messages = []provider.SimpleMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	for i := len(path) - 1; i >= 0; i-- {
		msg, ok := convMessages[path[i]]
		if !ok {
			break
		}
		messages = append(messages, provider.SimpleMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Image:   msg.Attachment,
		})
	}
	return messages
}
