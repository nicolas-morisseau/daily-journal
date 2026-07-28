package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"daily-journal/internal/journal"
)

// RenderSearchResult renders search matches grouped by entry, with the query
// highlighted inside each matching line.
func RenderSearchResult(theme *Theme, result journal.SearchResult, width int) string {
	var b strings.Builder

	header := theme.Title.Render("Search") + "  " + theme.Value.Render(result.Query)
	for _, tag := range result.Tags {
		header += "  " + theme.Badge.Render("#"+strings.TrimPrefix(tag, "#"))
	}
	b.WriteString(header + "\n")
	b.WriteString(theme.HRule(minInt(width, 60)) + "\n\n")

	if len(result.Matches) == 0 {
		b.WriteString(theme.Warning.Render("No matches found.") + "\n")
		if len(result.Tags) > 0 {
			b.WriteString("  " + theme.Muted.Render(fmt.Sprintf("Looked at %s carrying %s.",
				plural(result.Searched, "entry", "entries"), tagList(theme, result.Tags))) + "\n")
		}
		return b.String()
	}

	currentSlug := ""
	for _, match := range result.Matches {
		if match.Entry.Slug != currentSlug {
			if currentSlug != "" {
				b.WriteString("\n")
			}
			currentSlug = match.Entry.Slug
			b.WriteString("  " + theme.Accent.Render(match.Entry.Slug) + "\n")
		}
		for _, ctx := range match.Before {
			b.WriteString(contextRow(theme, ctx.LineNumber, ctx.Line, false, nil))
		}
		b.WriteString(contextRow(theme, match.LineNumber, match.Line, true, match.Ranges))
		for _, ctx := range match.After {
			b.WriteString(contextRow(theme, ctx.LineNumber, ctx.Line, false, nil))
		}
	}

	found := fmt.Sprintf("Found in %s", plural(result.EntryCount, "entry", "entries"))
	if len(result.Tags) > 0 {
		found += fmt.Sprintf(", of %s carrying %s",
			plural(result.Searched, "entry", "entries"), tagList(theme, result.Tags))
	}
	b.WriteString("\n  " + theme.Muted.Render(found) + "\n")
	return b.String()
}

// tagList renders the tags a search was restricted to, e.g. "#work and #bad".
func tagList(theme *Theme, tags []string) string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, "#"+strings.TrimPrefix(tag, "#"))
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// contextRow renders one gutter-numbered line of a search result.
func contextRow(theme *Theme, lineNumber int, line string, isMatch bool, ranges []journal.Range) string {
	gutter := theme.Muted.Render(fmt.Sprintf("%5d", lineNumber))
	marker := theme.Muted.Render(" │ ")
	body := strings.TrimRight(line, "\r")
	if isMatch {
		marker = theme.Accent.Render(" ▸ ")
		body = highlightRanges(theme, body, ranges)
	} else {
		body = theme.Muted.Render(body)
	}
	return "  " + gutter + marker + body + "\n"
}

// highlightRanges emphasises the matched byte ranges inside a line.
func highlightRanges(theme *Theme, line string, ranges []journal.Range) string {
	if len(ranges) == 0 {
		return line
	}
	var b strings.Builder
	last := 0
	for _, r := range ranges {
		if r.Start < last || r.End > len(line) {
			continue
		}
		b.WriteString(line[last:r.Start])
		b.WriteString(theme.Highlight.Render(line[r.Start:r.End]))
		last = r.End
	}
	b.WriteString(line[last:])
	return b.String()
}

// RenderCalendarStatic renders a calendar without the interactive chrome, for
// piped output.
func RenderCalendarStatic(theme *Theme, cal journal.Calendar) string {
	var b strings.Builder
	b.WriteString("\n  " + theme.Title.Render(cal.Month.Title()) + "\n\n")
	b.WriteString(Indent(RenderCalendarGrid(theme, cal, time.Time{})) + "\n\n")
	b.WriteString("  " + CalendarLegend(theme) + "\n\n")
	return b.String()
}

// Thousands formats an integer with thin separators, e.g. 12 345.
func Thousands(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, parts...), " ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
