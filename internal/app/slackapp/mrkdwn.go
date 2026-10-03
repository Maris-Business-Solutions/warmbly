package slackapp

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	mdBold     = regexp.MustCompile(`\*\*([^*\n]+?)\*\*|__([^_\n]+?)__`)
	mdItalic   = regexp.MustCompile(`(^|[^*\w])\*([^*\n]+?)\*`)
	mdStrike   = regexp.MustCompile(`~~([^~\n]+?)~~`)
	mdLink     = regexp.MustCompile(`\[([^\]\n]+)\]\((https?://[^)\s]+)\)`)
	mdHeading  = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*#*[ \t]*$`)
	mdBullet   = regexp.MustCompile(`(?m)^([ \t]*)[-*+][ \t]+`)
	boldMarker = "\x01"
)

// escapeMrkdwn escapes the three characters Slack reserves in text.
func escapeMrkdwn(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}

// toMrkdwn converts the assistant's Markdown to Slack mrkdwn for the plain
// text fallback: bold, italics, strikethrough, links, headings and bullets.
// Code spans and fences are escaped but otherwise left alone.
func toMrkdwn(md string) string {
	md = escapeMrkdwn(md)
	fences := strings.Split(md, "```")
	for i := range fences {
		if i%2 == 1 {
			continue
		}
		spans := strings.Split(fences[i], "`")
		for j := range spans {
			if j%2 == 0 {
				spans[j] = convertInline(spans[j])
			}
		}
		fences[i] = strings.Join(spans, "`")
	}
	return strings.Join(fences, "```")
}

func convertInline(s string) string {
	s = mdHeading.ReplaceAllString(s, boldMarker+"$1"+boldMarker)
	s = mdBold.ReplaceAllStringFunc(s, func(m string) string {
		return boldMarker + m[2:len(m)-2] + boldMarker
	})
	s = mdBullet.ReplaceAllString(s, "$1• ")
	s = mdItalic.ReplaceAllString(s, "${1}_${2}_")
	s = mdStrike.ReplaceAllString(s, "~$1~")
	s = mdLink.ReplaceAllString(s, "<$2|$1>")
	return strings.ReplaceAll(s, boldMarker, "*")
}

// truncateRunes caps s at n runes, ending with an ellipsis when it cuts.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n <= 1 {
		return string([]rune(s)[:n])
	}
	return strings.TrimRightFunc(string([]rune(s)[:n-1]), unicode.IsSpace) + "…"
}

// friendlyToolName turns a tool id into a label: "list_contacts" is "List contacts".
func friendlyToolName(name string) string {
	name = strings.TrimPrefix(name, "mcp_")
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == '.' })
	if len(words) == 0 {
		return "Tool"
	}
	out := strings.ToLower(strings.Join(words, " "))
	r, size := utf8.DecodeRuneInString(out)
	return string(unicode.ToUpper(r)) + out[size:]
}

// stripBotMention removes the bot's own mention from a message.
func stripBotMention(text, botUserID string) string {
	if botUserID != "" {
		text = strings.ReplaceAll(text, "<@"+botUserID+">", "")
	}
	return strings.TrimSpace(text)
}

// mentionsUser reports whether text mentions the given Slack user.
func mentionsUser(text, userID string) bool {
	return userID != "" && strings.Contains(text, "<@"+userID+">")
}
