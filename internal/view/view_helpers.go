package views

import (
	"html"
	"regexp"
	"strings"
)

// Archive snapshots also carry this markup, so only fixed presentation tags may pass through.
var questionMarkup = regexp.MustCompile(`</?(?:math|mfrac|mi|mn|mo|mrow|msqrt|msub|msubsup|msup|mtable|mtd|mtr)>|<math display="block">|<mo stretchy="false">`)

func renderQuestionContent(content, format string) string {
	if format != "mathml" {
		return html.EscapeString(content)
	}
	var result strings.Builder
	start := 0
	for _, tag := range questionMarkup.FindAllStringIndex(content, -1) {
		result.WriteString(html.EscapeString(html.UnescapeString(content[start:tag[0]])))
		result.WriteString(content[tag[0]:tag[1]])
		start = tag[1]
	}
	result.WriteString(html.EscapeString(html.UnescapeString(content[start:])))
	return result.String()
}
