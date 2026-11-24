package consignment

import "testing"

func TestNormaliseSeries(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"BLR", "BLR"},
		{"blr", "BLR"},
		{"  blr  ", "BLR"},
		{"HUB / 26-27", "HUB/26-27"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := NormaliseSeries(c.in); got != c.want {
				t.Errorf("NormaliseSeries(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCheckSeries(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"BLR", false},
		{"blr", false},
		{"HUB/26-27", false},
		{"A1", false},
		{"1", false},
		{"ABCDEFGHIJKLMNOP", false}, // sixteen characters
		{"ABCDEFGHIJKLMNOPQ", true}, // seventeen
		{"/BLR", true},              // cannot start with a separator
		{"BLR_NORTH", true},         // underscore is not allowed
		{"BLR.NORTH", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if err := CheckSeries(c.in); (err != nil) != c.wantErr {
				t.Errorf("CheckSeries(%q) = %v, want an error: %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestNewNumber(t *testing.T) {
	cases := []struct {
		name    string
		series  string
		seq     int
		wantErr bool
	}{
		{"the first number", "BLR", 1, false},
		{"no series", "", 1, false},
		{"the highest number", "BLR", SeqMax, false},
		{"zero is not a number", "BLR", 0, true},
		{"negative", "BLR", -1, true},
		{"past the highest", "BLR", SeqMax + 1, true},
		{"a series that cannot be one", "BLR_X", 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewNumber(c.series, c.seq)
			if c.wantErr {
				if err == nil {
					t.Fatalf("NewNumber(%q, %d) = %v, want an error", c.series, c.seq, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewNumber(%q, %d): %v", c.series, c.seq, err)
			}
			if got.Seq != c.seq {
				t.Errorf("sequence = %d, want %d", got.Seq, c.seq)
			}
		})
	}
}

func TestNumberFormat(t *testing.T) {
	cases := []struct {
		name   string
		number Number
		width  int
		want   string
	}{
		{"the default width", Number{Series: "BLR", Seq: 147}, 4, "BLR/0147"},
		{"the first number", Number{Series: "BLR", Seq: 1}, 4, "BLR/0001"},
		{"no series", Number{Seq: 147}, 4, "0147"},
		{"a wider pad", Number{Series: "BLR", Seq: 147}, 6, "BLR/000147"},
		{"a narrower pad than the number needs", Number{Series: "BLR", Seq: 1470}, 2, "BLR/1470"},
		{"a width of zero falls back to the default", Number{Series: "BLR", Seq: 147}, 0, "BLR/0147"},
		{"a width past the maximum is capped", Number{Series: "BLR", Seq: 147}, 99, "BLR/00000147"},
		{"a lower-case series comes out upper", Number{Series: "blr", Seq: 147}, 4, "BLR/0147"},
		{"no number at all", Number{Series: "BLR"}, 4, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.number.Format(c.width); got != c.want {
				t.Errorf("Format(%d) = %q, want %q", c.width, got, c.want)
			}
		})
	}

	if got, want := (Number{Series: "BLR", Seq: 147}).String(), "BLR/0147"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestNumberKeySortsByNumber(t *testing.T) {
	// The key is what the register indexes on, so a plain string sort of the
	// keys has to put number 9 before number 10.
	nine := Number{Series: "BLR", Seq: 9}.Key()
	ten := Number{Series: "BLR", Seq: 10}.Key()
	if !(nine < ten) {
		t.Errorf("key for 9 (%q) does not sort before the key for 10 (%q)", nine, ten)
	}
	lower := Number{Series: "blr", Seq: 9}.Key()
	if lower != nine {
		t.Errorf("the key depends on the case of the series: %q against %q", lower, nine)
	}
}

func TestParseNumber(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantSeries string
		wantSeq    int
		wantErr    bool
	}{
		{"as it is printed", "BLR/0147", "BLR", 147, false},
		{"with a hyphen", "BLR-147", "BLR", 147, false},
		{"a bare number", "147", "", 147, false},
		{"a padded bare number", "0147", "", 147, false},
		{"lower case", "blr/0147", "BLR", 147, false},
		{"a series with a separator in it", "HUB/26-27/0004", "HUB/26-27", 4, false},
		{"spaces around it", "  BLR/0147  ", "BLR", 147, false},
		{"empty", "", "", 0, true},
		{"nothing after the separator", "BLR/", "", 0, true},
		{"no number at the end", "HUB-NORTH", "", 0, true},
		{"zero", "BLR/0000", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseNumber(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseNumber(%q) = %v, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseNumber(%q): %v", c.in, err)
			}
			if got.Series != c.wantSeries || got.Seq != c.wantSeq {
				t.Errorf("ParseNumber(%q) = %q/%d, want %q/%d", c.in, got.Series, got.Seq, c.wantSeries, c.wantSeq)
			}
		})
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	for _, in := range []Number{
		{Series: "BLR", Seq: 1},
		{Series: "BLR", Seq: 9999},
		{Series: "HUB/26-27", Seq: 4},
		{Seq: 147},
	} {
		printed := in.Format(SeqWidthDefault)
		back, err := ParseNumber(printed)
		if err != nil {
			t.Fatalf("ParseNumber(%q): %v", printed, err)
		}
		if back != in {
			t.Errorf("%v printed as %q read back as %v", in, printed, back)
		}
	}
}

func TestBookNormalise(t *testing.T) {
	cases := []struct {
		name      string
		in        Book
		wantWidth int
		wantStart int
	}{
		{"an empty book gets the defaults", Book{}, SeqWidthDefault, 1},
		{"a width of its own is kept", Book{SeqWidth: 6}, 6, 1},
		{"a width past the maximum falls back", Book{SeqWidth: 99}, SeqWidthDefault, 1},
		{"a negative width falls back", Book{SeqWidth: -1}, SeqWidthDefault, 1},
		{"a start of its own is kept", Book{StartAt: 101}, SeqWidthDefault, 101},
		{"a start of zero becomes one", Book{StartAt: 0}, SeqWidthDefault, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalise()
			if got.SeqWidth != c.wantWidth {
				t.Errorf("width = %d, want %d", got.SeqWidth, c.wantWidth)
			}
			if got.StartAt != c.wantStart {
				t.Errorf("start = %d, want %d", got.StartAt, c.wantStart)
			}
		})
	}
}

func TestBookName(t *testing.T) {
	cases := []struct {
		in   Book
		want string
	}{
		{Book{Series: "BLR", Label: "Bengaluru booking office"}, "Bengaluru booking office"},
		{Book{Series: "BLR"}, "BLR"},
		{Book{}, "unlabelled series"},
	}
	for _, c := range cases {
		if got := c.in.Normalise().Name(); got != c.want {
			t.Errorf("Name() = %q, want %q", got, c.want)
		}
	}
}
