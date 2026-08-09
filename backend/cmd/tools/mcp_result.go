package tools

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// parseMCPToolResult turns an MCP CallToolResult into our ToolOutput.
// Text goes to Content; the first binary (image/audio/blob) is saved and
// returned as FileID so chat can attach it for the model.
func parseMCPToolResult(result *mcp.CallToolResult, user string) providers.ToolOutput {
	if result == nil {
		return providers.ToolOutput{Content: "Empty tool result."}
	}

	var textParts []string
	var fileID string
	var extraFileNotes []string

	if result.StructuredContent != nil {
		if b, err := json.Marshal(result.StructuredContent); err == nil && string(b) != "null" {
			textParts = append(textParts, string(b))
		}
	}

	for _, c := range result.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			if strings.TrimSpace(v.Text) != "" {
				textParts = append(textParts, v.Text)
			}

		case *mcp.ImageContent:
			id, note, err := persistMCPBinary(v.Data, v.MIMEType, "mcp-image", user)
			if err != nil {
				log.Error("Error saving MCP image content", "err", err)
				textParts = append(textParts, "[failed to save image from tool]")
				continue
			}
			fileID, extraFileNotes = takeFileID(fileID, id, note, extraFileNotes)

		case *mcp.AudioContent:
			id, note, err := persistMCPBinary(v.Data, v.MIMEType, "mcp-audio", user)
			if err != nil {
				log.Error("Error saving MCP audio content", "err", err)
				textParts = append(textParts, "[failed to save audio from tool]")
				continue
			}
			fileID, extraFileNotes = takeFileID(fileID, id, note, extraFileNotes)

		case *mcp.EmbeddedResource:
			if v.Resource == nil {
				continue
			}
			res := v.Resource
			if strings.TrimSpace(res.Text) != "" {
				textParts = append(textParts, res.Text)
			}
			if len(res.Blob) > 0 {
				name := resourceFileName(res.URI, res.MIMEType, "mcp-resource")
				id, note, err := persistMCPBinary(res.Blob, res.MIMEType, name, user)
				if err != nil {
					log.Error("Error saving MCP embedded resource", "err", err, "uri", res.URI)
					textParts = append(textParts, "[failed to save embedded resource from tool]")
					continue
				}
				fileID, extraFileNotes = takeFileID(fileID, id, note, extraFileNotes)
			} else if strings.TrimSpace(res.Text) == "" && res.URI != "" {
				textParts = append(textParts, formatResourceLink(res.URI, "", res.MIMEType, ""))
			}

		case *mcp.ResourceLink:
			textParts = append(textParts, formatResourceLink(v.URI, firstNonEmpty(v.Title, v.Name), v.MIMEType, v.Description))

		default:
			if c == nil {
				continue
			}
			if b, err := json.Marshal(c); err == nil {
				textParts = append(textParts, string(b))
			}
		}
	}

	textParts = append(textParts, extraFileNotes...)
	content := strings.TrimSpace(strings.Join(textParts, "\n"))
	if content == "" && fileID != "" {
		content = fmt.Sprintf("Tool returned a file (id: %s).", fileID)
	}
	if content == "" {
		content = "Tool returned no content."
	}
	if result.IsError {
		content = "Tool error: " + content
	}

	return providers.ToolOutput{Content: content, FileID: fileID}
}

func takeFileID(currentID, newID, note string, extras []string) (string, []string) {
	if currentID == "" {
		return newID, extras
	}
	// Single FileID slot: keep the first binary, mention the rest in text.
	return currentID, append(extras, note)
}

func persistMCPBinary(data []byte, mimeType, baseName, user string) (id, note string, err error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("empty binary content")
	}
	mimeType = strings.TrimSpace(strings.Split(mimeType, ";")[0])
	name := baseName
	if path.Ext(name) == "" {
		name = baseName + extFromMIME(mimeType)
	}
	f, err := saveBinaryFile(data, mimeType, name, user)
	if err != nil {
		return "", "", err
	}
	note = fmt.Sprintf("Saved file id=%s name=%s type=%s path=/%s", f.ID, f.Name, f.Type, f.Path)
	return f.ID, note, nil
}

func resourceFileName(uri, mimeType, fallback string) string {
	if uri != "" {
		base := path.Base(strings.Split(uri, "?")[0])
		if base != "" && base != "." && base != "/" {
			if path.Ext(base) == "" && mimeType != "" {
				base += extFromMIME(mimeType)
			}
			return base
		}
	}
	return fallback + extFromMIME(mimeType)
}

func formatResourceLink(uri, title, mimeType, description string) string {
	var b strings.Builder
	b.WriteString("Resource")
	if title != "" {
		b.WriteString(": ")
		b.WriteString(title)
	}
	if uri != "" {
		b.WriteString(" uri=")
		b.WriteString(uri)
	}
	if mimeType != "" {
		b.WriteString(" type=")
		b.WriteString(mimeType)
	}
	if description != "" {
		b.WriteString(" — ")
		b.WriteString(description)
	}
	return b.String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
