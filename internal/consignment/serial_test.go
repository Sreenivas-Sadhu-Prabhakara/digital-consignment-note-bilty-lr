package consignment

import (
	"strings"
	"testing"
	"time"
)

// at builds a note in a series with a number, a date and a status, which is all
// the serial audit looks at.
func at(series string, seq int, day int, status Status) Note {
	return Note{
		Number: Number{Series: series, Seq: seq},
		Date:   MustDate(2026, time.April, day),
		Status: status,
	}
}

func TestAuditACleanRunIsIntact(t *testing.T) {
	notes := []Note{
		at("BLR", 1, 1, StatusDelivered),
		at("BLR", 2, 2, StatusInTransit),
		at("BLR", 3, 3, StatusBooked),
		at("BLR", 4, 4, StatusCancelled),
		at("BLR", 5, 5, StatusVoid),
	}
	books := []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}}

	got := Audit(notes, books)
	if !got.Intact {
		t.Fatalf("a clean run reported broken: %s", got.Headline)
	}
	if got.Missing != 0 || got.Reused != 0 {
		t.Errorf("missing = %d and reused = %d, want nothing of either", got.Missing, got.Reused)
	}
	if len(got.Series) != 1 {
		t.Fatalf("got %d series, want 1", len(got.Series))
	}

	s := got.Series[0]
	if s.First != 1 || s.Last != 5 || s.Span != 5 || s.Accounted != 5 {
		t.Errorf("run = %d to %d, span %d, accounted %d; want 1 to 5, span 5, accounted 5",
			s.First, s.Last, s.Span, s.Accounted)
	}
	if s.Next != 6 {
		t.Errorf("next number = %d, want 6", s.Next)
	}
	// A cancelled note and a void number occupy their numbers but neither is an
	// issued consignment.
	if s.Issued != 3 || s.Delivered != 1 || s.Cancelled != 1 || s.Void != 1 {
		t.Errorf("issued %d, delivered %d, cancelled %d, void %d; want 3, 1, 1, 1",
			s.Issued, s.Delivered, s.Cancelled, s.Void)
	}
	if !strings.Contains(got.Headline, "intact") {
		t.Errorf("headline = %q, want it to say the series is intact", got.Headline)
	}
	if len(got.Breaks) != 0 {
		t.Errorf("breaks = %v, want none", got.Breaks)
	}
}

func TestAuditFindsGaps(t *testing.T) {
	cases := []struct {
		name        string
		seqs        []int
		startAt     int
		wantMissing int
		wantGaps    []string
	}{
		{
			name:        "one number missing in the middle",
			seqs:        []int{1, 2, 4, 5},
			startAt:     1,
			wantMissing: 1,
			wantGaps:    []string{"BLR/0003"},
		},
		{
			name:        "a run of numbers missing",
			seqs:        []int{1, 2, 6, 7},
			startAt:     1,
			wantMissing: 3,
			wantGaps:    []string{"BLR/0003 to BLR/0005"},
		},
		{
			name:        "two separate gaps",
			seqs:        []int{1, 3, 5},
			startAt:     1,
			wantMissing: 2,
			wantGaps:    []string{"BLR/0002", "BLR/0004"},
		},
		{
			name:        "the book starts below the first note",
			seqs:        []int{5, 6, 7},
			startAt:     1,
			wantMissing: 4,
			wantGaps:    []string{"BLR/0001 to BLR/0004"},
		},
		{
			name:        "the book starts at the first note",
			seqs:        []int{101, 102, 103},
			startAt:     101,
			wantMissing: 0,
			wantGaps:    nil,
		},
		{
			name:        "a book carried on from a paper one, with a hole",
			seqs:        []int{101, 103},
			startAt:     101,
			wantMissing: 1,
			wantGaps:    []string{"BLR/0102"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var notes []Note
			for i, seq := range c.seqs {
				notes = append(notes, at("BLR", seq, (i%28)+1, StatusBooked))
			}
			got := Audit(notes, []Book{{Series: "BLR", StartAt: c.startAt, SeqWidth: 4}})

			if got.Missing != c.wantMissing {
				t.Errorf("missing = %d, want %d (headline: %s)", got.Missing, c.wantMissing, got.Headline)
			}
			if (c.wantMissing > 0) == got.Intact {
				t.Errorf("intact = %v with %d missing", got.Intact, got.Missing)
			}

			var printed []string
			for _, g := range got.Series[0].Gaps {
				printed = append(printed, g.Printed)
			}
			if len(printed) != len(c.wantGaps) {
				t.Fatalf("gaps = %v, want %v", printed, c.wantGaps)
			}
			for i := range printed {
				if printed[i] != c.wantGaps[i] {
					t.Errorf("gap %d = %q, want %q", i, printed[i], c.wantGaps[i])
				}
			}
			if c.wantMissing > 0 && !strings.Contains(got.Headline, "BROKEN") {
				t.Errorf("headline = %q, want it to say the series is broken, loudly", got.Headline)
			}
		})
	}
}

func TestAuditFindsDuplicates(t *testing.T) {
	notes := []Note{
		at("BLR", 1, 1, StatusBooked),
		at("BLR", 2, 2, StatusBooked),
		at("BLR", 2, 3, StatusBooked), // the same number again
		at("BLR", 3, 4, StatusBooked),
	}
	got := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})

	if got.Intact {
		t.Fatal("a reused number reported the series intact")
	}
	if got.Missing != 0 {
		t.Errorf("missing = %d, want 0: nothing is missing, one number is doubled", got.Missing)
	}
	if got.Reused != 1 {
		t.Errorf("reused = %d, want 1", got.Reused)
	}

	s := got.Series[0]
	if len(s.Reuses) != 1 {
		t.Fatalf("reuses = %v, want one", s.Reuses)
	}
	if s.Reuses[0].Seq != 2 || s.Reuses[0].Count != 2 {
		t.Errorf("reuse = number %d on %d notes, want number 2 on 2", s.Reuses[0].Seq, s.Reuses[0].Count)
	}
	// The dates of both notes come back, so that the one to keep can be told
	// from the one to renumber.
	if len(s.Reuses[0].Dates) != 2 {
		t.Errorf("dates = %v, want one per note", s.Reuses[0].Dates)
	}
	if !strings.Contains(got.Headline, "BROKEN") || !strings.Contains(got.Headline, "more than once") {
		t.Errorf("headline = %q, want it to say a number is used more than once", got.Headline)
	}
	if len(got.Breaks) == 0 || !strings.Contains(got.Breaks[0], "cannot be used twice") {
		t.Errorf("breaks = %v, want the reuse named first", got.Breaks)
	}
}

func TestAuditFindsAGapAndAReuseTogether(t *testing.T) {
	notes := []Note{
		at("BLR", 1, 1, StatusBooked),
		at("BLR", 3, 2, StatusBooked),
		at("BLR", 3, 3, StatusBooked),
		at("BLR", 4, 4, StatusBooked),
	}
	got := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})

	if got.Intact {
		t.Fatal("reported intact")
	}
	if got.Missing != 1 {
		t.Errorf("missing = %d, want 1 (number 2)", got.Missing)
	}
	if got.Reused != 1 {
		t.Errorf("reused = %d, want 1 (number 3)", got.Reused)
	}
	if !strings.Contains(got.Headline, "missing") || !strings.Contains(got.Headline, "reused") {
		t.Errorf("headline = %q, want it to name both", got.Headline)
	}
	// The reuse is named before the gap: a number on two notes is worse than a
	// number on none.
	if len(got.Breaks) < 2 {
		t.Fatalf("breaks = %v, want at least two", got.Breaks)
	}
	if !strings.Contains(got.Breaks[0], "cannot be used twice") {
		t.Errorf("first break = %q, want the reuse", got.Breaks[0])
	}
}

func TestAuditFindsBackDating(t *testing.T) {
	notes := []Note{
		at("BLR", 1, 10, StatusBooked),
		at("BLR", 2, 5, StatusBooked), // numbered later, dated earlier
		at("BLR", 3, 11, StatusBooked),
	}
	got := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})

	// The numbers are all there and each is used once, so the series is intact.
	// The dates are still worth saying something about.
	if !got.Intact {
		t.Errorf("intact = false; the numbers run unbroken, so it should be true. %s", got.Headline)
	}
	s := got.Series[0]
	if len(s.OutOfOrder) != 1 {
		t.Fatalf("out of order = %v, want one", s.OutOfOrder)
	}
	if s.OutOfOrder[0].Seq != 2 || s.OutOfOrder[0].AfterSeq != 1 {
		t.Errorf("out of order = %d after %d, want 2 after 1", s.OutOfOrder[0].Seq, s.OutOfOrder[0].AfterSeq)
	}
	if !strings.Contains(got.Headline, "out of date order") {
		t.Errorf("headline = %q, want it to mention the dates", got.Headline)
	}
}

func TestAuditIgnoresVoidNumbersWhenCheckingDateOrder(t *testing.T) {
	// A voided number carries no consignment, so its date says nothing about
	// the order notes were written in.
	notes := []Note{
		at("BLR", 1, 10, StatusBooked),
		at("BLR", 2, 1, StatusVoid),
		at("BLR", 3, 11, StatusBooked),
	}
	got := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})
	if len(got.Series[0].OutOfOrder) != 0 {
		t.Errorf("out of order = %v, want none: the early date is on a void number", got.Series[0].OutOfOrder)
	}
	if !got.Intact {
		t.Errorf("intact = false, want true: %s", got.Headline)
	}
}

func TestAuditSeveralSeriesAreAuditedSeparately(t *testing.T) {
	notes := []Note{
		at("BLR", 1, 1, StatusBooked),
		at("BLR", 2, 2, StatusBooked),
		at("HUB", 1, 3, StatusBooked),
		at("HUB", 3, 4, StatusBooked), // a hole in the second book only
	}
	books := []Book{
		{Series: "BLR", Label: "Bengaluru", StartAt: 1, SeqWidth: 4},
		{Series: "HUB", Label: "Hubballi", StartAt: 1, SeqWidth: 4},
	}
	got := Audit(notes, books)

	if len(got.Series) != 2 {
		t.Fatalf("got %d series, want 2", len(got.Series))
	}
	if !got.Series[0].Intact {
		t.Errorf("the first book reported broken: %s", got.Series[0].Verdict)
	}
	if got.Series[1].Intact {
		t.Error("the second book reported intact despite a hole")
	}
	if got.Intact {
		t.Error("the register reported intact with one book broken")
	}
	if got.Missing != 1 {
		t.Errorf("missing across the register = %d, want 1", got.Missing)
	}
}

func TestAuditTreatsSeriesLabelsCaseInsensitively(t *testing.T) {
	// A clerk typing "blr" meant the same book as one typing "BLR". A register
	// that treated them as two books would show no gap in either.
	notes := []Note{
		at("blr", 1, 1, StatusBooked),
		at("BLR", 3, 2, StatusBooked),
	}
	got := Audit(notes, nil)

	if len(got.Series) != 1 {
		t.Fatalf("got %d series, want 1: the labels differ only in case", len(got.Series))
	}
	if got.Missing != 1 {
		t.Errorf("missing = %d, want 1", got.Missing)
	}
}

func TestAuditOnAnEmptyRegister(t *testing.T) {
	got := Audit(nil, nil)
	if !got.Intact {
		t.Error("an empty register reported broken")
	}
	if got.Notes != 0 || len(got.Series) != 0 {
		t.Errorf("notes = %d and series = %d, want nothing of either", got.Notes, len(got.Series))
	}
	if !strings.Contains(got.Headline, "No consignment note") {
		t.Errorf("headline = %q", got.Headline)
	}
}

func TestAuditOnADeclaredButEmptyBook(t *testing.T) {
	got := Audit(nil, []Book{{Series: "BLR", StartAt: 101, SeqWidth: 4}})
	if !got.Intact {
		t.Error("an empty book reported broken")
	}
	if len(got.Series) != 1 {
		t.Fatalf("got %d series, want 1", len(got.Series))
	}
	if got.Series[0].Next != 101 {
		t.Errorf("next number = %d, want the book's own start of 101", got.Series[0].Next)
	}
	if !strings.Contains(got.Series[0].Verdict, "No note has been issued") {
		t.Errorf("verdict = %q", got.Series[0].Verdict)
	}
}

func TestAuditSkipsUnnumberedNotes(t *testing.T) {
	// A note being typed in has no number yet. It is a finding on the note, not
	// a hole in a series.
	notes := []Note{
		at("BLR", 1, 1, StatusBooked),
		{Number: Number{Series: "BLR", Seq: 0}, Date: MustDate(2026, time.April, 2), Status: StatusBooked},
		at("BLR", 2, 3, StatusBooked),
	}
	got := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})
	if !got.Intact {
		t.Errorf("intact = false, want true: %s", got.Headline)
	}
	if got.Notes != 2 {
		t.Errorf("numbered notes = %d, want 2", got.Notes)
	}
}

func TestNextNumberNeverFillsAGap(t *testing.T) {
	// Slotting a note into a hole out of date order would make the register
	// say two wrong things at once. Allocation always goes past the highest.
	notes := []Note{
		at("BLR", 1, 1, StatusBooked),
		at("BLR", 5, 2, StatusBooked),
	}
	books := []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}}

	got := NextNumber(notes, books, "BLR")
	if got.Seq != 6 {
		t.Errorf("NextNumber = %d, want 6 even though 2, 3 and 4 are free", got.Seq)
	}
	if got.Series != "BLR" {
		t.Errorf("series = %q, want BLR", got.Series)
	}
}

func TestNextNumberOnANewSeries(t *testing.T) {
	cases := []struct {
		name   string
		notes  []Note
		books  []Book
		series string
		want   int
	}{
		{"nothing at all", nil, nil, "BLR", 1},
		{"a declared book that starts high", nil, []Book{{Series: "BLR", StartAt: 101}}, "BLR", 101},
		{"a series nobody declared", []Note{at("BLR", 4, 1, StatusBooked)}, nil, "BLR", 5},
		{"a second series with the first one filled", []Note{at("BLR", 4, 1, StatusBooked)}, []Book{{Series: "HUB", StartAt: 1}}, "HUB", 1},
		{"case-insensitive", []Note{at("BLR", 4, 1, StatusBooked)}, nil, "blr", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextNumber(c.notes, c.books, c.series); got.Seq != c.want {
				t.Errorf("NextNumber = %d, want %d", got.Seq, c.want)
			}
		})
	}
}

func TestGapNumbersListsTheIndividualHoles(t *testing.T) {
	notes := []Note{at("BLR", 1, 1, StatusBooked), at("BLR", 6, 2, StatusBooked)}
	audit := Audit(notes, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})

	got := GapNumbers(audit.Series[0], 0)
	want := []string{"2", "3", "4", "5"}
	if len(got) != len(want) {
		t.Fatalf("GapNumbers = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("GapNumbers[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if capped := GapNumbers(audit.Series[0], 2); len(capped) != 2 {
		t.Errorf("GapNumbers with a limit of 2 gave %v", capped)
	}
}

func TestSeriesOf(t *testing.T) {
	notes := []Note{at("HUB", 1, 1, StatusBooked), at("BLR", 1, 2, StatusBooked)}
	books := []Book{{Series: "BLR"}}

	got := SeriesOf(notes, books)
	// The declared books come first, in the order they were declared; then any
	// series only the notes know about.
	if len(got) != 2 || got[0] != "BLR" || got[1] != "HUB" {
		t.Errorf("SeriesOf = %v, want [BLR HUB]", got)
	}
}

func TestWidthOf(t *testing.T) {
	books := []Book{{Series: "BLR", SeqWidth: 6}, {Series: "HUB"}}
	cases := []struct {
		series string
		want   int
	}{
		{"BLR", 6},
		{"blr", 6},
		{"HUB", SeqWidthDefault},
		{"XXX", SeqWidthDefault},
	}
	for _, c := range cases {
		if got := WidthOf(books, c.series); got != c.want {
			t.Errorf("WidthOf(%q) = %d, want %d", c.series, got, c.want)
		}
	}
}

func TestAuditOnTheWorkedRegisterIsIntact(t *testing.T) {
	got := Audit(SampleNotes(), []Book{SampleBook()})
	if !got.Intact {
		t.Fatalf("the worked register is broken, which defeats the point of it: %s\n%v", got.Headline, got.Breaks)
	}
	if got.Series[0].Next != 6 {
		t.Errorf("next number in the worked register = %d, want 6", got.Series[0].Next)
	}
}

func TestHeadlineOnDeclaredButEmptyBooks(t *testing.T) {
	// A register whose books are declared and empty has no series to be
	// broken, and saying it is intact would be a boast about nothing.
	got := Audit(nil, []Book{{Series: "BLR", StartAt: 1}, {Series: "HUB", StartAt: 1}})
	if !got.Intact {
		t.Error("empty books reported broken")
	}
	if !strings.Contains(got.Headline, "No consignment note has been issued yet") {
		t.Errorf("headline = %q", got.Headline)
	}
}
