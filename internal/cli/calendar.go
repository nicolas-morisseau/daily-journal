package cli

import (
	"os/exec"
	"time"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// calendarSource adapts the journal and the configured editor to what the
// calendar view needs. The view stays free of filesystem and editor knowledge.
type calendarSource struct {
	app *App
}

// Calendar implements ui.CalendarSource.
func (s calendarSource) Calendar(m journal.Month) journal.Calendar {
	return s.app.Journal.Calendar(m)
}

// Today implements ui.CalendarSource.
func (s calendarSource) Today() time.Time { return s.app.Journal.Today() }

// EntryContent implements ui.CalendarSource.
func (s calendarSource) EntryContent(day time.Time) (string, bool) {
	return s.app.Journal.EntryContent(day)
}

// Insight implements ui.CalendarSource.
func (s calendarSource) Insight(day time.Time, opts journal.InsightOptions) journal.Insight {
	return s.app.Journal.Insight(day, opts)
}

// EditCommand implements ui.CalendarSource: it creates the entry when it does
// not exist yet and returns the editor ready to run in the foreground.
func (s calendarSource) EditCommand(day time.Time) (*exec.Cmd, error) {
	ed, err := s.app.resolveEditor()
	if err != nil {
		return nil, err
	}
	path, _, err := s.app.Journal.CreateEntry(journal.NewEntryOptions{
		Date: day.Format(dateparse.Layout),
	})
	if err != nil {
		return nil, err
	}
	return ed.Cmd(path), nil
}

// calendar backs `journal calendar [YYYY-MM]`.
//
// On a terminal it runs the full-screen view; piped, it prints the month grid so
// the command stays usable in a script.
func (a *App) calendar(monthArg string) error {
	month := journal.MonthOf(a.Journal.Today())
	if monthArg != "" {
		parsed, err := journal.ParseMonth(monthArg)
		if err != nil {
			return usagef("%s", err.Error())
		}
		month = parsed
	}

	if !ui.Interactive() {
		a.Printer.Printf("%s", ui.RenderCalendarStatic(a.Theme, a.Journal.Calendar(month)))
		return nil
	}
	return ui.ShowCalendar(a.Theme, calendarSource{app: a}, month)
}
