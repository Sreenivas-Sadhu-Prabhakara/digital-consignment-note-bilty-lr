package consignment

import "testing"

func TestPaiseFormat(t *testing.T) {
	cases := []struct {
		name string
		in   Paise
		want string
	}{
		{"zero", 0, "0.00"},
		{"one paisa", 1, "0.01"},
		{"ten paise", 10, "0.10"},
		{"ninety-nine paise", 99, "0.99"},
		{"one rupee", 100, "1.00"},
		{"under a thousand", 99900, "999.00"},
		{"exactly a thousand", 100000, "1,000.00"},
		{"ten thousand", 1000000, "10,000.00"},
		{"a lakh groups in pairs", 10000000, "1,00,000.00"},
		{"twelve lakh fifty thousand", 125000000, "12,50,000.00"},
		{"a crore", 1000000000, "1,00,00,000.00"},
		{"with paise", 1234567, "12,345.67"},
		{"negative", -125050, "-1,250.50"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Format(); got != c.want {
				t.Errorf("Format() = %q, want %q", got, c.want)
			}
			if got := c.in.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParsePaise(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Paise
		wantErr bool
	}{
		{"empty is nothing", "", 0, false},
		{"whole rupees", "1250", 125000, false},
		{"one decimal place fills out", "1250.5", 125050, false},
		{"two decimal places", "1250.50", 125050, false},
		{"grouped the Indian way", "12,50,000", 125000000, false},
		{"with the symbol", "₹1,250.75", 125075, false},
		{"leading decimal point", ".75", 75, false},
		{"zero", "0", 0, false},
		{"negative", "-500", -50000, false},
		{"plus sign", "+500", 50000, false},
		{"three decimal places is refused", "1250.505", 0, true},
		{"not a number", "twelve", 0, true},
		{"sign alone", "-", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePaise(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParsePaise(%q) = %d, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePaise(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParsePaise(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestPaiseParseFormatRoundTrip(t *testing.T) {
	for _, amount := range []Paise{0, 1, 99, 100, 12345, 125000000, -125050} {
		formatted := amount.Format()
		back, err := ParsePaise(formatted)
		if err != nil {
			t.Fatalf("ParsePaise(%q): %v", formatted, err)
		}
		if back != amount {
			t.Errorf("%d formatted as %q read back as %d", amount, formatted, back)
		}
	}
}

func TestPctString(t *testing.T) {
	cases := []struct {
		in   Pct
		want string
	}{
		{0, "0%"},
		{500, "5%"},
		{1200, "12%"},
		{1250, "12.5%"},
		{1805, "18.05%"},
		{10000, "100%"},
		{-500, "-5%"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.in.String(); got != c.want {
				t.Errorf("Pct(%d).String() = %q, want %q", int64(c.in), got, c.want)
			}
		})
	}
}

func TestPctOf(t *testing.T) {
	cases := []struct {
		name string
		rate Pct
		base Paise
		want Paise
	}{
		{"five percent of a thousand rupees", 500, 100000, 5000},
		{"twelve percent of a thousand rupees", 1200, 100000, 12000},
		{"zero rate", 0, 100000, 0},
		{"zero base", 500, 0, 0},
		{"rounds half away from zero, up", 500, 1050, 53},   // 52.5 paise
		{"rounds half away from zero, down", 500, 1030, 52}, // 51.5 paise -> 52
		{"half a percent", 50, 100000, 500},
		{"twelve and a half percent", 1250, 100000, 12500},
		{"negative base rounds away from zero", 500, -1050, -53},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rate.Of(c.base); got != c.want {
				t.Errorf("Pct(%d).Of(%d) = %d, want %d", int64(c.rate), c.base, got, c.want)
			}
		})
	}
}

func TestParsePct(t *testing.T) {
	cases := []struct {
		in      string
		want    Pct
		wantErr bool
	}{
		{"", 0, false},
		{"5", 500, false},
		{"5%", 500, false},
		{"12", 1200, false},
		{"12.5", 1250, false},
		{"12.50", 1250, false},
		{"0", 0, false},
		{"18.05", 1805, false},
		{"12.505", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParsePct(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParsePct(%q) = %d, want an error", c.in, int64(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePct(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParsePct(%q) = %d, want %d", c.in, int64(got), int64(c.want))
			}
		})
	}
}

func TestDivRound(t *testing.T) {
	cases := []struct {
		name     string
		num, den int64
		want     int64
	}{
		{"exact", 10, 2, 5},
		{"rounds up at the half", 5, 2, 3},
		{"rounds up above the half", 7, 4, 2},
		{"rounds down below the half", 5, 4, 1},
		{"negative rounds away from zero", -5, 2, -3},
		{"negative denominator", 5, -2, -3},
		{"both negative", -5, -2, 3},
		{"zero numerator", 0, 7, 0},
		{"zero denominator gives zero, not a panic", 7, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := divRound(c.num, c.den); got != c.want {
				t.Errorf("divRound(%d, %d) = %d, want %d", c.num, c.den, got, c.want)
			}
		})
	}
}

func TestPaiseRupees(t *testing.T) {
	cases := []struct {
		in   Paise
		want int64
	}{
		{0, 0},
		{99, 0},
		{100, 1},
		{199, 1},
		{150000, 1500},
	}
	for _, c := range cases {
		if got := c.in.Rupees(); got != c.want {
			t.Errorf("Paise(%d).Rupees() = %d, want %d", c.in, got, c.want)
		}
	}
}
