package journal

import (
	"fmt"
	"time"

	"daily-journal/internal/dateparse"
)

// Month identifies a calendar month.
type Month struct {
	Year  int
	Month time.Month
}

// MonthOf returns the month containing t.
func MonthOf(t time.Time) Month { return Month{Year: t.Year(), Month: t.Month()} }

// ParseMonth parses a `YYYY-MM` string.
func ParseMonth(s string) (Month, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Month{}, fmt.Errorf("invalid month %q, expected YYYY-MM", s)
	}
	return MonthOf(t), nil
}

// String formats the month as YYYY-MM.
func (m Month) String() string { return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month)) }

// Title formats the month for display, e.g. "January 2026".
func (m Month) Title() string { return fmt.Sprintf("%s %d", m.Month.String(), m.Year) }

// firstDay returns the first day of the month at midnight.
func (m Month) firstDay(loc *time.Location) time.Time {
	return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, loc)
}

// Days returns the number of days in the month.
func (m Month) Days(loc *time.Location) int {
	return m.firstDay(loc).AddDate(0, 1, -1).Day()
}

// Add returns the month offset by n months.
func (m Month) Add(n int) Month {
	return MonthOf(m.firstDay(time.UTC).AddDate(0, n, 0))
}

// Day is one cell of a rendered calendar.
type Day struct {
	// Date is the day; the zero value marks a leading padding cell.
	Date time.Time
	// HasEntry is true when a `YYYY-MM-DD.md` entry exists for the day.
	HasEntry bool
	// IsToday marks the current day.
	IsToday bool
	// Words is the entry's word count, 0 when there is no entry.
	Words int
}

// Padding reports whether the cell is empty padding before the 1st.
func (d Day) Padding() bool { return d.Date.IsZero() }

// Calendar is a month laid out as weeks starting on Monday.
type Calendar struct {
	Month Month
	// Weeks holds 7-cell rows; the first and last may contain padding cells.
	Weeks [][]Day
}

// Calendar builds the calendar for a month, marking days that have an entry.
func (j *Journal) Calendar(m Month) Calendar {
	loc := j.Now().Location()
	today := j.Today()
	first := m.firstDay(loc)

	// Monday-first offset for the 1st of the month.
	lead := (int(first.Weekday()) + 6) % 7

	cells := make([]Day, 0, lead+m.Days(loc))
	for i := 0; i < lead; i++ {
		cells = append(cells, Day{})
	}
	for day := 1; day <= m.Days(loc); day++ {
		date := time.Date(m.Year, m.Month, day, 0, 0, 0, 0, loc)
		path := j.PathFor(date.Format(dateparse.Layout))
		cell := Day{Date: date, IsToday: date.Equal(today), HasEntry: Exists(path)}
		if cell.HasEntry {
			cell.Words = WordCountFile(path)
		}
		cells = append(cells, cell)
	}
	// Pad the final week so every row has seven cells.
	for len(cells)%7 != 0 {
		cells = append(cells, Day{})
	}

	cal := Calendar{Month: m}
	for i := 0; i < len(cells); i += 7 {
		cal.Weeks = append(cal.Weeks, cells[i:i+7])
	}
	return cal
}

// WeekdayHeaders are the Monday-first column labels.
var WeekdayHeaders = [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
