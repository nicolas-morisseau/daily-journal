package journal

import (
	"strings"
)

// Match is a single matching line with surrounding context.
type Match struct {
	// Entry is the entry the match belongs to.
	Entry Entry
	// LineNumber is the 1-indexed line of the match.
	LineNumber int
	// Line is the matching line.
	Line string
	// Ranges are the byte offsets of the query inside Line, for highlighting.
	Ranges []Range
	// Before and After hold the context lines around the match.
	Before []ContextLine
	After  []ContextLine
}

// Range is a half-open byte interval inside a line.
type Range struct {
	Start, End int
}

// ContextLine is a non-matching line shown around a match.
type ContextLine struct {
	LineNumber int
	Line       string
}

// SearchResult groups matches by entry, preserving entry order.
type SearchResult struct {
	Query string
	// Tags are the tags the search was restricted to, as given.
	Tags    []string
	Matches []Match
	// EntryCount is the number of distinct entries containing a match.
	EntryCount int
	// Searched is how many entries were actually looked at, which is fewer than
	// the journal holds once tags narrow it down.
	Searched int
}

// SearchOptions describes what to look for.
type SearchOptions struct {
	// Query is the text to find, case-insensitively. Required.
	Query string
	// Tags restricts the search to entries carrying every one of these tags.
	// A leading `#` is accepted and ignored. Empty means no restriction.
	Tags []string
}

// ContextLines is how many lines of context surround each match, matching the
// bash implementation's `grep -A2 -B2`.
const ContextLines = 2

// Search runs a case-insensitive substring search over the text of the entries,
// optionally restricted to those carrying a set of tags.
//
// The text search and the tag filter are deliberately separate: the query looks
// at the words, the tags look at the note as a whole.
func (j *Journal) Search(opts SearchOptions) (SearchResult, error) {
	result := SearchResult{Query: opts.Query, Tags: opts.Tags}
	if opts.Query == "" {
		return result, nil
	}
	entries, err := j.EntriesSorted(OldestFirst)
	if err != nil {
		return result, err
	}

	wanted := normaliseTags(opts.Tags)
	needle := strings.ToLower(opts.Query)
	for _, entry := range entries {
		content, err := Read(entry.Path)
		if err != nil {
			continue
		}
		if !hasAllTags(content, wanted) {
			continue
		}
		result.Searched++
		lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		matchedEntry := false
		for i, line := range lines {
			ranges := findAll(strings.ToLower(line), needle)
			if len(ranges) == 0 {
				continue
			}
			matchedEntry = true
			result.Matches = append(result.Matches, Match{
				Entry:      entry,
				LineNumber: i + 1,
				Line:       line,
				Ranges:     ranges,
				Before:     contextSlice(lines, i-ContextLines, i),
				After:      contextSlice(lines, i+1, i+1+ContextLines),
			})
		}
		if matchedEntry {
			result.EntryCount++
		}
	}
	return result, nil
}

func contextSlice(lines []string, from, to int) []ContextLine {
	if from < 0 {
		from = 0
	}
	if to > len(lines) {
		to = len(lines)
	}
	var out []ContextLine
	for i := from; i < to; i++ {
		out = append(out, ContextLine{LineNumber: i + 1, Line: lines[i]})
	}
	return out
}

// findAll returns every occurrence of needle in haystack. Both must already be
// lowercased for a case-insensitive search.
func findAll(haystack, needle string) []Range {
	var ranges []Range
	for offset := 0; ; {
		idx := strings.Index(haystack[offset:], needle)
		if idx < 0 {
			return ranges
		}
		start := offset + idx
		ranges = append(ranges, Range{Start: start, End: start + len(needle)})
		offset = start + len(needle)
	}
}

// normaliseTags lowercases the requested tags and drops any leading `#`, so the
// comparison matches what collectTags produces.
func normaliseTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	wanted := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tag), "#")))
		if tag != "" {
			wanted = append(wanted, tag)
		}
	}
	return wanted
}

// hasAllTags reports whether an entry carries every requested tag. Adding a tag
// narrows the search, so the tags combine with AND.
//
// Matching is exact on the tag word, using the same detection as the calendar's
// tag summary: #work does not match #workshop, and `[#bad]` counts.
func hasAllTags(content string, wanted []string) bool {
	if len(wanted) == 0 {
		return true
	}
	present := collectTags(content)
	for _, tag := range wanted {
		if !present[tag] {
			return false
		}
	}
	return true
}
