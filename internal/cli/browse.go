package cli

import (
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// search backs `journal search [--tag TAG]... QUERY`.
func (a *App) search(query string, tags []string) error {
	if _, err := a.entriesOrHint(journal.OldestFirst); err != nil {
		return err
	}
	result, err := a.Journal.Search(journal.SearchOptions{Query: query, Tags: tags})
	if err != nil {
		return err
	}
	a.Printer.Blank()
	a.Printer.Printf("%s", ui.RenderSearchResult(a.Theme, result, ui.ContentWidth()))
	return nil
}

// week backs `journal week`: open every entry of the current week at once.
func (a *App) week() error {
	entries, err := a.Journal.WeekEntries(a.Journal.Today())
	if err != nil {
		return err
	}

	p, t := a.Printer, a.Theme
	p.Blank()
	p.Println("  " + t.Title.Render("This week's entries"))
	p.Blank()
	if len(entries) == 0 {
		p.Muted("  No entries this week")
		p.Blank()
		return nil
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
		p.Println("  " + t.Accent.Render("•") + " " + t.Value.Render(entry.Slug) + "  " +
			t.Muted.Render(entry.Date.Format("Monday")))
	}
	p.Blank()

	ed, err := a.resolveEditor()
	if err != nil {
		return err
	}
	return ed.Open(paths...)
}
