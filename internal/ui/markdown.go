package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	boldRe     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe   = regexp.MustCompile(`(^|[^*])\*([^*]+)\*`)
	inlineCode = regexp.MustCompile("`([^`]+)`")
	tagRe      = regexp.MustCompile(`(^|\s)(#[\p{L}\p{N}_-]+)`)
	bulletRe   = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	todoRe     = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s+(.*)$`)
	quoteRe    = regexp.MustCompile(`^>\s?(.*)$`)
	hrRe       = regexp.MustCompile(`^\s*(-{3,}|\*{3,}|_{3,})\s*$`)
)

// RenderMarkdown gives markdown a light coat of colour for terminal previews.
//
// It is deliberately not a full markdown engine: entries are plain text that
// happens to use a few markdown conventions, and the point is to keep them
// readable rather than to reflow them.
func RenderMarkdown(theme *Theme, content string, width int) string {
	if width < 20 {
		width = 20
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	out := make([]string, 0, len(lines))

	inFence := false
	code := lipgloss.NewStyle().Foreground(theme.Palette.Info)

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			out = append(out, theme.Muted.Render(line))
			continue
		}
		if inFence {
			out = append(out, code.Render(line))
			continue
		}
		out = append(out, renderLine(theme, line, width))
	}
	return strings.Join(out, "\n")
}

func renderLine(theme *Theme, line string, width int) string {
	switch {
	case strings.TrimSpace(line) == "":
		return ""

	case hrRe.MatchString(line):
		return theme.HRule(width)

	case headingRe.MatchString(line):
		m := headingRe.FindStringSubmatch(line)
		text := renderInline(theme, m[2])
		if len(m[1]) == 1 {
			return theme.Title.Render(text)
		}
		return lipgloss.NewStyle().Bold(true).Foreground(theme.Palette.Info).Render(text)

	case todoRe.MatchString(line):
		m := todoRe.FindStringSubmatch(line)
		box := theme.Muted.Render("☐")
		body := renderInline(theme, m[3])
		if m[2] != " " {
			box = theme.Success.Render("☑")
			body = lipgloss.NewStyle().Faint(true).Render(m[3])
		}
		return m[1] + box + " " + body

	case bulletRe.MatchString(line):
		m := bulletRe.FindStringSubmatch(line)
		return m[1] + theme.Accent.Render("•") + " " + renderInline(theme, m[2])

	case quoteRe.MatchString(line):
		m := quoteRe.FindStringSubmatch(line)
		return theme.Rule.Render("▏") + " " + theme.Muted.Render(m[1])

	default:
		return renderInline(theme, line)
	}
}

// renderInline styles bold, italic, inline code and #tags.
func renderInline(theme *Theme, text string) string {
	bold := lipgloss.NewStyle().Bold(true)
	italic := lipgloss.NewStyle().Italic(true)
	code := lipgloss.NewStyle().Foreground(theme.Palette.Info)

	text = inlineCode.ReplaceAllStringFunc(text, func(m string) string {
		return code.Render(strings.Trim(m, "`"))
	})
	text = boldRe.ReplaceAllStringFunc(text, func(m string) string {
		return bold.Render(strings.Trim(m, "*"))
	})
	text = italicRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := italicRe.FindStringSubmatch(m)
		return sub[1] + italic.Render(sub[2])
	})
	text = tagRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := tagRe.FindStringSubmatch(m)
		return sub[1] + theme.Accent.Render(sub[2])
	})
	return text
}
