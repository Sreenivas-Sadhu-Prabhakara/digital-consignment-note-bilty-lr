package consignment

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Date is a calendar day with no time and no zone. A consignment note is dated
// by the day it was issued and delivered on the day the receiver signed for it;
// neither has an hour, and a timestamp carrying one would make a note booked
// late on the 31st in one zone fall on the 1st in another.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// ErrNoDate is returned where a date is needed and none was given.
var ErrNoDate = fmt.Errorf("no date")

// NewDate makes a date, refusing one the calendar does not have. The 31st of
// February is not a date that can exist on a consignment note.
func NewDate(year int, month time.Month, day int) (Date, error) {
	if year < 1900 || year > 2200 {
		return Date{}, fmt.Errorf("%d is not a year a consignment note is dated in", year)
	}
	if month < time.January || month > time.December {
		return Date{}, fmt.Errorf("%d is not a month", int(month))
	}
	if day < 1 || day > DaysInMonth(year, month) {
		return Date{}, fmt.Errorf("%s %d has no day %d", month, year, day)
	}
	return Date{Year: year, Month: month, Day: day}, nil
}

// MustDate is NewDate for a date written into the program itself.
func MustDate(year int, month time.Month, day int) Date {
	d, err := NewDate(year, month, day)
	if err != nil {
		panic(err)
	}
	return d
}

// Today is the day the machine thinks it is.
func Today() Date {
	now := time.Now()
	return Date{Year: now.Year(), Month: now.Month(), Day: now.Day()}
}

// IsZero reports whether no date was given.
func (d Date) IsZero() bool { return d.Year == 0 && d.Month == 0 && d.Day == 0 }

// String writes the date the way it is stored and sorted: 2026-04-07.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// Print writes the date the way a consignment note prints it: 07-04-2026, the
// order a transport office reads a date in.
func (d Date) Print() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%02d-%02d-%04d", d.Day, int(d.Month), d.Year)
}

// Long writes the date in words for a proof of delivery, where a figure that
// could be read two ways is worth spelling out: 7 April 2026.
func (d Date) Long() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d %s %d", d.Day, d.Month, d.Year)
}

// Time is the date as midnight UTC, for arithmetic only.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// ParseDate reads a date written as 2026-04-07 or 07-04-2026 or 07/04/2026.
// An empty string is the zero date and not an error: a consignment note being
// typed in has no delivery date yet.
func ParseDate(s string) (Date, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Date{}, nil
	}

	sep := "-"
	if strings.Contains(t, "/") {
		sep = "/"
	}
	parts := strings.Split(t, sep)
	if len(parts) != 3 {
		return Date{}, fmt.Errorf("%q is not a date; write it as 2026-04-07 or 07-04-2026", s)
	}

	var a, b, c int
	if _, err := fmt.Sscanf(parts[0]+" "+parts[1]+" "+parts[2], "%d %d %d", &a, &b, &c); err != nil {
		return Date{}, fmt.Errorf("%q is not a date; write it as 2026-04-07 or 07-04-2026", s)
	}

	if len(parts[0]) == 4 {
		return NewDate(a, time.Month(b), c)
	}
	return NewDate(c, time.Month(b), a)
}

// MarshalJSON writes the date as a string, so the data file stays readable.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`""`), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON reads a date from a string.
func (d *Date) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("a date has to be written as a string: %w", err)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// DaysInMonth is how many days a month has, counting February correctly.
func DaysInMonth(year int, month time.Month) int {
	switch month {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		if IsLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

// IsLeapYear applies the Gregorian rule in full: every fourth year, except
// every hundredth, except every four-hundredth. 2000 was a leap year and 1900
// and 2100 are not.
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// AddDays moves a date forward or back by whole days, carrying across months
// and years and February correctly.
func (d Date) AddDays(days int) Date {
	if d.IsZero() {
		return d
	}
	t := d.Time().AddDate(0, 0, days)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// DaysUntil is how many whole days separate this date from a later one.
// Negative where the other date is earlier.
func (d Date) DaysUntil(other Date) int {
	if d.IsZero() || other.IsZero() {
		return 0
	}
	return int(other.Time().Sub(d.Time()).Hours() / 24)
}

// Before, After and Equal order two dates. The zero date orders before every
// real one, which is what a register sorting unissued rows wants.
func (d Date) Before(other Date) bool { return d.cmp(other) < 0 }
func (d Date) After(other Date) bool  { return d.cmp(other) > 0 }
func (d Date) Equal(other Date) bool  { return d.cmp(other) == 0 }

func (d Date) cmp(other Date) int {
	switch {
	case d.Year != other.Year:
		return sign(d.Year - other.Year)
	case d.Month != other.Month:
		return sign(int(d.Month) - int(other.Month))
	default:
		return sign(d.Day - other.Day)
	}
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	default:
		return 0
	}
}

// FinancialYear is the Indian financial year a date falls in, written the way a
// consignment note series is labelled: 2026-27 for a date between 1 April 2026
// and 31 March 2027. A transport office starts a fresh number series every
// financial year, and the label is what tells two series apart.
func (d Date) FinancialYear() string {
	if d.IsZero() {
		return ""
	}
	start := d.Year
	if d.Month < time.April {
		start--
	}
	return fmt.Sprintf("%d-%02d", start, (start+1)%100)
}
