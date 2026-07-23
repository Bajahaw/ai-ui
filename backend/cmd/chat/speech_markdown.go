package chat

import "regexp"

// Precompiled markdown strippers for TTS (see stripMarkdownForTTS).
// Note: Go's RE2 engine does not support backreferences.
var (
	fencedCodeRe  = regexp.MustCompile("(?s)```[\\w-]*\\n?(.*?)```")
	inlineCodeRe  = regexp.MustCompile("`([^`]+)`")
	imageMdRe     = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	linkMdRe      = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	headingMdRe   = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	boldMdRe      = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	italicMdRe    = regexp.MustCompile(`\*(.+?)\*|_(.+?)_`)
	strikeMdRe    = regexp.MustCompile(`~~(.*?)~~`)
	quoteListMdRe = regexp.MustCompile(`(?m)^(?:\s*[-*+]\s+|\s*\d+\.\s+|>\s*)`)
	hrMdRe        = regexp.MustCompile(`(?m)^(?:-{3,}|\*{3,}|_{3,})\s*$`)
	whitespaceRe  = regexp.MustCompile(`\s+`)
)
