package consignment

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIsLeapYear(t *testing.T) {
	cases := []struct {
		year int
		want bool
	}{
		{2024, true},
		{2025, false},
		{2026, false},
		{2027, false},
		{2028, true},
		{1900, false}, // divisible by 100 but not 400
		{2000, true},  // divisible by 400
		{2100, false},
		{2400, true},
	}
	for _, c := range cases {
		if got := IsLeapYear(c.year); got != c.want {
			t.Errorf("IsLeapYear(%d) = %v, want %v", c.year, got, c.want)
		}
	}
}

func TestDaysInMonth(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.January, 31},
		{2026, time.February, 28},
		{2024, time.February, 29},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2026, time.March, 31},
		{2026, time.April, 30},
		{2026, time.June, 30},
		{2026, time.September, 30},
		{2026, time.November, 30},
		{2026, time.December, 31},
	}
	for _, c := range cases {
		if got := DaysInMonth(c.year, c.month); got != c.want {
			t.Errorf("DaysInMonth(%d, %s) = %d, want %d", c.year, c.month, got, c.want)
		}
	}
}

func TestNewDateRefusesDatesTheCalendarHasNot(t *testing.T) {
	cases := []struct {
		name    string
		year    int
		month   time.Month
		day     int
		wantErr bool
	}{
		{"an ordinary date", 2026, time.April, 7, false},
		{"the end of a long month", 2026, time.January, 31, false},
		{"the end of a short month", 2026, time.April, 30, false},
		{"31 April does not exist", 2026, time.April, 31, true},
		{"29 February in a common year", 2026, time.February, 29, true},
		{"29 February in a leap year", 2024, time.February, 29, false},
		{"30 February never", 2024, time.February, 30, true},
		{"day zero", 2026, time.April, 0, true},
		{"month zero", 2026, time.Month(0), 7, true},
		{"month thirteen", 2026, time.Month(13), 7, true},
		{"a year out of range", 1850, time.April, 7, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewDate(c.year, c.month, c.day)
			if (err != nil) != c.wantErr {
				t.Errorf("NewDate(%d, %s, %d) error = %v, want an error: %v", c.year, c.month, c.day, err, c.wantErr)
			}
		})
	}
}

func TestDateAddDaysCrossesMonthsAndLeapYears(t *testing.T) {
	cases := []struct {
		name string
		from Date
		days int
		want Date
	}{
		{"within a month", MustDate(2026, time.April, 7), 3, MustDate(2026, time.April, 10)},
		{"across a month end", MustDate(2026, time.April, 29), 3, MustDate(2026, time.May, 2)},
		{"across February in a common year", MustDate(2026, time.February, 27), 3, MustDate(2026, time.March, 2)},
		{"across February in a leap year", MustDate(2024, time.February, 27), 3, MustDate(2024, time.March, 1)},
		{"onto 29 February in a leap year", MustDate(2024, time.February, 27), 2, MustDate(2024, time.February, 29)},
		{"across a year end", MustDate(2026, time.December, 30), 3, MustDate(2027, time.January, 2)},
		{"backwards across a month start", MustDate(2026, time.March, 2), -3, MustDate(2026, time.February, 27)},
		{"zero days", MustDate(2026, time.April, 7), 0, MustDate(2026, time.April, 7)},
		{"a long way forward", MustDate(2026, time.April, 7), 365, MustDate(2027, time.April, 7)},
		{"a year across a leap day", MustDate(2024, time.January, 1), 365, MustDate(2024, time.December, 31)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.from.AddDays(c.days); got != c.want {
				t.Errorf("%s.AddDays(%d) = %s, want %s", c.from, c.days, got, c.want)
			}
		})
	}
}

func TestDateAddDaysOnTheZeroDateStaysZero(t *testing.T) {
	if got := (Date{}).AddDays(5); !got.IsZero() {
		t.Errorf("the zero date moved to %s", got)
	}
}

func TestDateDaysUntil(t *testing.T) {
	cases := []struct {
		name string
		from Date
		to   Date
		want int
	}{
		{"same day", MustDate(2026, time.April, 7), MustDate(2026, time.April, 7), 0},
		{"next day", MustDate(2026, time.April, 7), MustDate(2026, time.April, 8), 1},
		{"across February in a common year", MustDate(2026, time.February, 27), MustDate(2026, time.March, 1), 2},
		{"across February in a leap year", MustDate(2024, time.February, 27), MustDate(2024, time.March, 1), 3},
		{"backwards", MustDate(2026, time.April, 10), MustDate(2026, time.April, 7), -3},
		{"a zero date gives nothing", Date{}, MustDate(2026, time.April, 7), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.from.DaysUntil(c.to); got != c.want {
				t.Errorf("%s.DaysUntil(%s) = %d, want %d", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Date
		wantErr bool
	}{
		{"empty is the zero date", "", Date{}, false},
		{"sortable form", "2026-04-07", MustDate(2026, time.April, 7), false},
		{"the form a transport office writes", "07-04-2026", MustDate(2026, time.April, 7), false},
		{"with slashes", "07/04/2026", MustDate(2026, time.April, 7), false},
		{"sortable with slashes", "2026/04/07", MustDate(2026, time.April, 7), false},
		{"a leap day", "2024-02-29", MustDate(2024, time.February, 29), false},
		{"a day the calendar has not", "2026-02-29", Date{}, true},
		{"31 April", "2026-04-31", Date{}, true},
		{"two parts only", "2026-04", Date{}, true},
		{"not a date", "last Tuesday", Date{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %s, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseDate(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestDatePrintForms(t *testing.T) {
	d := MustDate(2026, time.April, 7)
	if got, want := d.String(), "2026-04-07"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got, want := d.Print(), "07-04-2026"; got != want {
		t.Errorf("Print() = %q, want %q", got, want)
	}
	if got, want := d.Long(), "7 April 2026"; got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
	for _, got := range []string{(Date{}).String(), (Date{}).Print(), (Date{}).Long()} {
		if got != "" {
			t.Errorf("the zero date printed %q, want empty", got)
		}
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type holder struct {
		On Date `json:"on"`
	}
	cases := []struct {
		name string
		in   Date
		want string
	}{
		{"an ordinary date", MustDate(2026, time.April, 7), `{"on":"2026-04-07"}`},
		{"a leap day", MustDate(2024, time.February, 29), `{"on":"2024-02-29"}`},
		{"the zero date", Date{}, `{"on":""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(holder{On: c.in})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(raw) != c.want {
				t.Fatalf("Marshal = %s, want %s", raw, c.want)
			}
			var back holder
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if back.On != c.in {
				t.Errorf("round trip gave %s, want %s", back.On, c.in)
			}
		})
	}
}

func TestDateJSONRefusesADateTheCalendarHasNot(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2026-02-30"`), &d); err == nil {
		t.Fatal("30 February was read without an error")
	}
	if err := json.Unmarshal([]byte(`7`), &d); err == nil {
		t.Fatal("a number was read as a date")
	}
}

func TestDateOrdering(t *testing.T) {
	earlier := MustDate(2026, time.April, 7)
	later := MustDate(2026, time.April, 8)
	nextMonth := MustDate(2026, time.May, 1)
	nextYear := MustDate(2027, time.January, 1)

	cases := []struct {
		name   string
		a, b   Date
		before bool
		after  bool
		same   bool
	}{
		{"a day apart", earlier, later, true, false, false},
		{"the same day", earlier, earlier, false, false, true},
		{"the other way", later, earlier, false, true, false},
		{"a month apart", later, nextMonth, true, false, false},
		{"a year apart", nextMonth, nextYear, true, false, false},
		{"the zero date orders first", Date{}, earlier, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.Before(c.b); got != c.before {
				t.Errorf("%s.Before(%s) = %v, want %v", c.a, c.b, got, c.before)
			}
			if got := c.a.After(c.b); got != c.after {
				t.Errorf("%s.After(%s) = %v, want %v", c.a, c.b, got, c.after)
			}
			if got := c.a.Equal(c.b); got != c.same {
				t.Errorf("%s.Equal(%s) = %v, want %v", c.a, c.b, got, c.same)
			}
		})
	}
}

func TestFinancialYear(t *testing.T) {
	cases := []struct {
		in   Date
		want string
	}{
		{MustDate(2026, time.April, 1), "2026-27"},
		{MustDate(2026, time.March, 31), "2025-26"},
		{MustDate(2026, time.December, 31), "2026-27"},
		{MustDate(2027, time.January, 1), "2026-27"},
		{MustDate(1999, time.December, 31), "1999-00"},
		{Date{}, ""},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.in.FinancialYear(); got != c.want {
				t.Errorf("%s.FinancialYear() = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
