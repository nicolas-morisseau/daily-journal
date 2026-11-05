package cli

import (
	"os/exec"
	"time"

	"daily-journal/internal/dateparse"
	"daily-journal/internal/journal"
	"daily-journal/internal/ui"
)

// viewSource adapts the journal and the configured editor to what the interactive
// views need. It satisfies ui.CalendarSource and ui.ReviewSource, which keeps
// both views free of filesystem and editor knowledge.
type viewSource struct {
	app *App
}

func (s viewSource) Calendar(m journal.Month) journal.Calendar {
	return s.app.Journal.Calendar(m)
}

func (s viewSource) Today() time.Time { return s.app.Journal.Today() }

func (s viewSource) EntryContent(day time.Time) (string, bool) {
	return s.app.Journal.EntryContent(day)
}

func (s viewSource) Insight(day time.Time, opts journal.InsightOptions) journal.Insight {
	return s.app.Journal.Insight(day, opts)
}

func (s viewSource) Review(opts journal.ReviewOptions) (journal.Review, error) {
	return s.app.Journal.Review(opts)
}

// ReviewPeriod is the period to review around a day: the sprint it belongs to
// when SPRINT_START says where sprints start, the fortnight ending on it
// otherwise.
func (s viewSource) ReviewPeriod(day time.Time) (journal.ReviewOptions, error) {
	anchor, err := s.app.sprintAnchor()
	if err != nil {
		return journal.ReviewOptions{}, err
	}
	return journal.ReviewOptions{To: day, Anchor: anchor}, nil
}

func (s viewSource) EditCommand(day time.Time) (*exec.Cmd, error) {
	return s.app.editCommand(day)
}

// editCommand prepares the editor for a day's entry, creating the file when it
// does not exist yet. The caller runs it in the foreground.
func (a *App) editCommand(day time.Time) (*exec.Cmd, error) {
	ed, err := a.resolveEditor()
	if err != nil {
		return nil, err
	}
	path, _, err := a.Journal.CreateEntry(journal.NewEntryOptions{
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
	return ui.ShowCalendar(a.Theme, viewSource{app: a}, month, a.Config.Mouse)
}
