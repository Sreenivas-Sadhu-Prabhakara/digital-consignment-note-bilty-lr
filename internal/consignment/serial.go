package consignment

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Serial integrity is what a consignment note register is for.
//
// A bilty book is printed with its numbers already on the leaves. Every leaf is
// accounted for: used, spoiled and struck through, or still in the book. That
// is the whole discipline, and it is what makes the register evidence — if the
// numbers run unbroken, a consignment that was carried cannot be missing from
// the register, and a note that turns up cannot have been written after the
// fact and slotted in.
//
// Typing numbers into a spreadsheet throws the discipline away silently, which
// is worse than losing the book: the holes do not show. So the register is
// audited on every read, and the audit is loud. Two things break it:
//
//   - a gap, where a number in the run was never accounted for; and
//   - a reuse, where two notes carry the same number.
//
// A third thing is suspicious without being a break: a note with a higher
// number dated before a note with a lower one, which is what back-dating looks
// like.
//
// A gap is closed by accounting for the number, not by renumbering. A spoiled
// leaf is recorded as a void entry that occupies its number and says why. A
// consignment that was booked and then called off is cancelled, and keeps its
// number too. Nothing is ever deleted and nothing is ever renumbered, because
// either would leave the register saying something that is not true.

// Break is what kind of thing the audit found.
type Break string

const (
	// BreakGap is one or more numbers in the run that nothing accounts for.
	BreakGap Break = "gap"
	// BreakReuse is one number carried by more than one note.
	BreakReuse Break = "reuse"
	// BreakOutOfOrder is a higher number dated before a lower one.
	BreakOutOfOrder Break = "out-of-order"
)

// Gap is a run of missing numbers in a series.
type Gap struct {
	Series string `json:"series"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Count  int    `json:"count"`
	// Printed is the gap as it reads on screen: "BLR/0147" for one number, or
	// "BLR/0147 to BLR/0151" for a run.
	Printed string `json:"printed"`
}

// Reuse is one number carried by more than one note.
type Reuse struct {
	Series string `json:"series"`
	Seq    int    `json:"seq"`
	Count  int    `json:"count"`
	// Dates is the date on each of the notes carrying the number, so that the
	// one to keep can be told from the one to renumber.
	Dates   []string `json:"dates"`
	Printed string   `json:"printed"`
}

// OutOfOrder is a note numbered after another but dated before it.
type OutOfOrder struct {
	Series   string `json:"series"`
	Seq      int    `json:"seq"`
	Date     Date   `json:"date"`
	AfterSeq int    `json:"afterSeq"`
	Date2    Date   `json:"afterDate"`
	Printed  string `json:"printed"`
}

// SeriesAudit is one number series, accounted for.
type SeriesAudit struct {
	Series   string `json:"series"`
	Label    string `json:"label"`
	SeqWidth int    `json:"seqWidth"`

	// First and Last are the lowest and highest numbers accounted for. Where
	// the book says it starts higher or lower than the notes in it do, First is
	// the book's own start — a book that starts at 101 and whose first note is
	// 105 is missing 101 to 104, and the audit has to say so.
	First int `json:"first"`
	Last  int `json:"last"`
	// Span is how many numbers the run covers, and Accounted is how many of
	// them something accounts for. They are equal where the series is intact.
	Span      int `json:"span"`
	Accounted int `json:"accounted"`
	Missing   int `json:"missing"`

	// How the accounted-for numbers were used.
	Issued    int `json:"issued"`
	Delivered int `json:"delivered"`
	Cancelled int `json:"cancelled"`
	Void      int `json:"void"`

	Gaps       []Gap        `json:"gaps"`
	Reuses     []Reuse      `json:"reuses"`
	OutOfOrder []OutOfOrder `json:"outOfOrder"`

	// Intact is true only where there is no gap and no reuse. An out-of-order
	// date is reported but does not of itself break the series, because the
	// numbers are all there and each is used once.
	Intact bool `json:"intact"`
	// Next is the number the next note gets: one past the highest accounted
	// for. A gap is never filled by allocation, because slotting a note into a
	// hole out of date order makes the register say the wrong thing twice over.
	Next int `json:"next"`

	Verdict string `json:"verdict"`
}

// RegisterAudit is every series in the register, accounted for.
type RegisterAudit struct {
	Series []SeriesAudit `json:"series"`

	Notes     int `json:"notes"`
	Span      int `json:"span"`
	Accounted int `json:"accounted"`
	Missing   int `json:"missing"`
	Reused    int `json:"reused"`

	Intact bool `json:"intact"`
	// Headline is the one line the register puts at the top of the screen. It
	// says the series is intact, or it says what is wrong with it, and it is
	// deliberately blunt.
	Headline string `json:"headline"`
	// Breaks is every break across every series, worst first, for the banner.
	Breaks []string `json:"breaks"`
}

// Audit accounts for every number in the register.
//
// The books are what the office says its series are; notes in a series that no
// book declares are still audited, because a note that exists has a number
// whether the book was set up or not.
func Audit(notes []Note, books []Book) RegisterAudit {
	declared := map[string]Book{}
	order := []string{}
	for _, b := range books {
		b = b.Normalise()
		if _, seen := declared[b.Series]; !seen {
			order = append(order, b.Series)
		}
		declared[b.Series] = b
	}

	// Group the notes by series, and within a series by number.
	bySeries := map[string]map[int][]Note{}
	for _, n := range notes {
		series := NormaliseSeries(n.Number.Series)
		if n.Number.Seq <= 0 {
			// A note with no number is not part of any series run. It is a
			// validation finding on the note itself, not a hole in a series.
			continue
		}
		if _, seen := bySeries[series]; !seen {
			bySeries[series] = map[int][]Note{}
			if _, isDeclared := declared[series]; !isDeclared {
				order = append(order, series)
			}
		}
		bySeries[series][n.Number.Seq] = append(bySeries[series][n.Number.Seq], n)
	}

	out := RegisterAudit{Intact: true}
	for _, series := range order {
		audit := auditSeries(series, declared[series], bySeries[series])
		out.Series = append(out.Series, audit)

		out.Span += audit.Span
		out.Accounted += audit.Accounted
		out.Missing += audit.Missing
		for _, r := range audit.Reuses {
			out.Reused += r.Count - 1
		}
		if !audit.Intact {
			out.Intact = false
		}
	}
	out.Notes = countNumbered(notes)
	out.Breaks, out.Headline = headline(out)
	return out
}

// auditSeries accounts for one series.
func auditSeries(series string, book Book, byNumber map[int][]Note) SeriesAudit {
	out := SeriesAudit{
		Series:   series,
		Label:    book.Label,
		SeqWidth: book.Normalise().SeqWidth,
		Intact:   true,
	}
	if out.Label == "" {
		out.Label = Book{Series: series}.Name()
	}

	seqs := make([]int, 0, len(byNumber))
	for seq := range byNumber {
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)

	// An empty series that the office has declared is intact and waiting: its
	// next number is the book's own start.
	if len(seqs) == 0 {
		out.First, out.Last, out.Span = 0, 0, 0
		out.Next = book.Normalise().StartAt
		out.Verdict = "No note has been issued in this series yet."
		return out
	}

	// The run starts at the book's declared start where there is one, so that a
	// book starting at 101 whose first note is 105 shows four missing numbers
	// rather than a tidy run beginning at 105.
	out.First = seqs[0]
	if start := book.Normalise().StartAt; start > 0 && len(byNumber) > 0 && start < out.First {
		out.First = start
	}
	out.Last = seqs[len(seqs)-1]
	out.Span = out.Last - out.First + 1
	out.Accounted = len(seqs)
	out.Missing = out.Span - out.Accounted
	out.Next = out.Last + 1

	for _, seq := range seqs {
		carried := byNumber[seq]
		if len(carried) > 1 {
			dates := make([]string, 0, len(carried))
			for _, n := range carried {
				label := n.Date.Print()
				if label == "" {
					label = "undated"
				}
				dates = append(dates, label)
			}
			sort.Strings(dates)
			out.Reuses = append(out.Reuses, Reuse{
				Series:  series,
				Seq:     seq,
				Count:   len(carried),
				Dates:   dates,
				Printed: Number{Series: series, Seq: seq}.Format(out.SeqWidth),
			})
			out.Intact = false
		}

		for _, n := range carried {
			switch n.Status {
			case StatusDelivered:
				out.Issued++
				out.Delivered++
			case StatusCancelled:
				out.Cancelled++
			case StatusVoid:
				out.Void++
			default:
				out.Issued++
			}
		}
	}

	// The gaps, as runs rather than one line per missing number: a book that
	// lost a whole pad reads better as "0210 to 0259" than as fifty lines.
	present := map[int]bool{}
	for _, seq := range seqs {
		present[seq] = true
	}
	runStart := 0
	for seq := out.First; seq <= out.Last+1; seq++ {
		missing := seq <= out.Last && !present[seq]
		switch {
		case missing && runStart == 0:
			runStart = seq
		case !missing && runStart != 0:
			out.Gaps = append(out.Gaps, newGap(series, runStart, seq-1, out.SeqWidth))
			out.Intact = false
			runStart = 0
		}
	}

	// Numbers that go up while dates go down. Only consecutive accounted-for
	// numbers are compared, which is enough to find a back-dated note without
	// flooding the screen once one is found.
	var prev Note
	var havePrev bool
	for _, seq := range seqs {
		carried := byNumber[seq]
		n := carried[0]
		if n.Date.IsZero() || n.Status == StatusVoid {
			continue
		}
		if havePrev && n.Date.Before(prev.Date) {
			out.OutOfOrder = append(out.OutOfOrder, OutOfOrder{
				Series:   series,
				Seq:      n.Number.Seq,
				Date:     n.Date,
				AfterSeq: prev.Number.Seq,
				Date2:    prev.Date,
				Printed: fmt.Sprintf("%s is dated %s but %s before it is dated %s",
					Number{Series: series, Seq: n.Number.Seq}.Format(out.SeqWidth), n.Date.Print(),
					Number{Series: series, Seq: prev.Number.Seq}.Format(out.SeqWidth), prev.Date.Print()),
			})
		}
		prev, havePrev = n, true
	}

	out.Verdict = seriesVerdict(out)
	return out
}

func newGap(series string, from, to, width int) Gap {
	g := Gap{Series: series, From: from, To: to, Count: to - from + 1}
	first := Number{Series: series, Seq: from}.Format(width)
	if from == to {
		g.Printed = first
	} else {
		g.Printed = first + " to " + Number{Series: series, Seq: to}.Format(width)
	}
	return g
}

// seriesVerdict is the one sentence the series gets.
func seriesVerdict(a SeriesAudit) string {
	if a.Intact {
		return fmt.Sprintf("Intact: %s accounted for, unbroken from %s to %s.",
			plural(a.Accounted, "number", "numbers"),
			Number{Series: a.Series, Seq: a.First}.Format(a.SeqWidth),
			Number{Series: a.Series, Seq: a.Last}.Format(a.SeqWidth))
	}

	var parts []string
	if a.Missing > 0 {
		parts = append(parts, plural(a.Missing, "number is missing", "numbers are missing"))
	}
	if len(a.Reuses) > 0 {
		parts = append(parts, plural(len(a.Reuses), "number is used twice or more", "numbers are used twice or more"))
	}
	return "Broken: " + joinAnd(parts) + "."
}

// headline is the blunt line at the top of the register and the list of breaks
// behind it.
func headline(a RegisterAudit) ([]string, string) {
	var breaks []string
	for _, s := range a.Series {
		for _, r := range s.Reuses {
			breaks = append(breaks, fmt.Sprintf("%s is on %s — a consignment note number cannot be used twice. Renumber all but one of them to the end of the series.",
				r.Printed, plural(r.Count, "note", "notes")))
		}
	}
	for _, s := range a.Series {
		for _, g := range s.Gaps {
			breaks = append(breaks, fmt.Sprintf("%s is missing — %s unaccounted for. Record what became of the leaf: a void entry if it was spoiled, a cancelled note if the consignment was called off, or the note itself if it was issued and never entered.",
				g.Printed, plural(g.Count, "number", "numbers")))
		}
	}
	for _, s := range a.Series {
		for _, o := range s.OutOfOrder {
			breaks = append(breaks, o.Printed+" — the numbers run forward but the dates run back, which is what a note written after the event looks like.")
		}
	}

	switch {
	case a.Accounted == 0:
		// Declared books with nothing in them yet. There is no series to be
		// broken, so saying it is intact would be a boast about nothing.
		return breaks, "No consignment note has been issued yet."
	case a.Missing > 0 && a.Reused > 0:
		return breaks, fmt.Sprintf("The number series is BROKEN: %s missing and %s reused.",
			plural(a.Missing, "number", "numbers"), plural(a.Reused, "number", "numbers"))
	case a.Missing > 0:
		return breaks, fmt.Sprintf("The number series is BROKEN: %s unaccounted for.",
			plural(a.Missing, "number is", "numbers are"))
	case a.Reused > 0:
		return breaks, fmt.Sprintf("The number series is BROKEN: %s used more than once.",
			plural(a.Reused, "number is", "numbers are"))
	case len(breaks) > 0:
		return breaks, fmt.Sprintf("The numbers run unbroken, but %s out of date order.",
			plural(len(breaks), "note is", "notes are"))
	default:
		return breaks, fmt.Sprintf("The number series is intact: %s accounted for, no gaps and no reuse.",
			plural(a.Accounted, "number", "numbers"))
	}
}

// NextNumber is the number the next note in a series gets.
func NextNumber(notes []Note, books []Book, series string) Number {
	series = NormaliseSeries(series)
	audit := Audit(notes, books)
	for _, s := range audit.Series {
		if s.Series == series {
			return Number{Series: series, Seq: s.Next}
		}
	}

	start := 1
	for _, b := range books {
		if NormaliseSeries(b.Series) == series {
			start = b.Normalise().StartAt
			break
		}
	}
	return Number{Series: series, Seq: start}
}

// SeriesOf is every series the register has anything in, in order, so the
// screen can offer them.
func SeriesOf(notes []Note, books []Book) []string {
	seen := map[string]bool{}
	var out []string
	for _, b := range books {
		s := NormaliseSeries(b.Series)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, n := range notes {
		s := NormaliseSeries(n.Number.Series)
		if n.Number.Seq > 0 && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func countNumbered(notes []Note) int {
	n := 0
	for _, note := range notes {
		if note.Number.Seq > 0 {
			n++
		}
	}
	return n
}

// joinAnd joins a list the way a sentence does: "a", "a and b", "a, b and c".
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

// WidthOf is the padding a series prints its numbers at, taken from the book
// that declares it or the default where none does.
func WidthOf(books []Book, series string) int {
	series = NormaliseSeries(series)
	for _, b := range books {
		if NormaliseSeries(b.Series) == series {
			return b.Normalise().SeqWidth
		}
	}
	return SeqWidthDefault
}

// GapNumbers lists the individual missing numbers in a series, up to a limit,
// so the screen can offer each one to be accounted for.
func GapNumbers(a SeriesAudit, limit int) []string {
	if limit <= 0 {
		limit = 50
	}
	var out []string
	for _, g := range a.Gaps {
		for seq := g.From; seq <= g.To; seq++ {
			if len(out) >= limit {
				return out
			}
			out = append(out, strconv.Itoa(seq))
		}
	}
	return out
}
