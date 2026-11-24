package consignment

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A consignment note is identified by its number, and the number is the thing
// the whole register hangs off. A paper bilty book is printed with the numbers
// already on the leaves, in triplicate, so a leaf cannot be used twice and a
// missing leaf shows as a hole. Typing the numbers in by hand throws that away,
// so the number here is not a free-text field: it is a series and a sequence,
// and the two of them together are what the register is audited on.

const (
	// SeqWidthDefault is how many digits the sequence is padded to on the
	// printed note. Four digits is a bilty book's convention and leaves room
	// for a year of booking at a small office.
	SeqWidthDefault = 4
	// SeqWidthMax is as wide as the padding goes.
	SeqWidthMax = 8
	// SeqMax is the highest sequence this tool will allocate. Past it the
	// series has to be changed, which is what a transport office does at the
	// turn of the financial year anyway.
	SeqMax = 99999999
)

// seriesShape is what a series label may be made of: letters, digits, a hyphen
// or a slash. "BLR", "HUB/26-27", "A1". It is part of the printed number, so it
// cannot carry spaces or anything that would read as two numbers.
var seriesShape = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/-]{0,15}$`)

// Number is a consignment note number: which series it belongs to and where it
// sits in that series.
type Number struct {
	Series string `json:"series"`
	Seq    int    `json:"seq"`
}

// NormaliseSeries puts a series label in the form it is stored and compared in:
// upper case, trimmed, with the spaces taken out. Two series that differ only
// in case are the same series, because a clerk typing "blr" and "BLR" meant the
// same book and a register that treated them as two would show no gaps in
// either.
func NormaliseSeries(s string) string {
	out := strings.ToUpper(strings.TrimSpace(s))
	out = strings.ReplaceAll(out, " ", "")
	return out
}

// CheckSeries reports whether a series label can be used. An empty label is
// allowed and means the office keeps one unlabelled book.
func CheckSeries(s string) error {
	norm := NormaliseSeries(s)
	if norm == "" {
		return nil
	}
	if !seriesShape.MatchString(norm) {
		return fmt.Errorf("%q cannot be a series label: use letters, digits, a hyphen or a slash, up to sixteen characters", s)
	}
	return nil
}

// NewNumber makes a number, refusing a sequence that cannot be one.
func NewNumber(series string, seq int) (Number, error) {
	if err := CheckSeries(series); err != nil {
		return Number{}, err
	}
	if seq <= 0 {
		return Number{}, fmt.Errorf("a consignment note number starts at 1, and %d was given", seq)
	}
	if seq > SeqMax {
		return Number{}, fmt.Errorf("%d is past the highest number this register keeps (%d); start a new series", seq, SeqMax)
	}
	return Number{Series: NormaliseSeries(series), Seq: seq}, nil
}

// IsZero reports whether no number has been allocated.
func (n Number) IsZero() bool { return n.Seq == 0 }

// Format writes the number the way it is printed on the note: the series, a
// slash, and the sequence padded to the given width. With no series it is just
// the padded sequence.
func (n Number) Format(width int) string {
	if n.Seq <= 0 {
		return ""
	}
	if width < 1 {
		width = SeqWidthDefault
	}
	if width > SeqWidthMax {
		width = SeqWidthMax
	}

	seq := leftPad(strconv.Itoa(n.Seq), width)
	series := NormaliseSeries(n.Series)
	if series == "" {
		return seq
	}
	return series + "/" + seq
}

// String writes the number at the default width.
func (n Number) String() string { return n.Format(SeqWidthDefault) }

// Key is how a number is compared and indexed, with the sequence at a fixed
// width so that sorting the keys sorts the numbers.
func (n Number) Key() string {
	return NormaliseSeries(n.Series) + "#" + leftPad(strconv.Itoa(n.Seq), SeqWidthMax)
}

// ParseNumber reads a number written the way it is printed: "BLR/0147",
// "BLR-147", "147". The series is whatever came before the last separator and
// the sequence is the digits after it. A number with no digits at all is an
// error, because a register cannot audit a number it cannot order.
func ParseNumber(s string) (Number, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Number{}, fmt.Errorf("no consignment note number was given")
	}

	// Split at the last separator, so "HUB/26-27/0004" keeps "HUB/26-27" as
	// the series and takes "0004" as the sequence.
	cut := strings.LastIndexAny(t, "/-")
	series, digits := "", t
	if cut >= 0 {
		series, digits = t[:cut], t[cut+1:]
	}

	digits = strings.TrimSpace(digits)
	if digits == "" {
		return Number{}, fmt.Errorf("%q ends with a separator and no number", s)
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			// The last separator was part of the series after all, as in a
			// bare "HUB-NORTH" with no sequence on it.
			return Number{}, fmt.Errorf("%q does not end in a number; write it as BLR/0147", s)
		}
	}

	seq, err := strconv.Atoi(digits)
	if err != nil {
		return Number{}, fmt.Errorf("%q does not end in a number; write it as BLR/0147", s)
	}
	return NewNumber(series, seq)
}

// Book is a numbered series of consignment notes — one bilty book. A transport
// office keeps one per branch, or one per financial year, or both.
type Book struct {
	Series   string `json:"series"`
	Label    string `json:"label"`
	SeqWidth int    `json:"seqWidth"`
	// StartAt is the first number in the book. An office moving off a paper
	// book carries on from where the paper one stopped rather than starting
	// again at one, so this is not always 1.
	StartAt int `json:"startAt"`
}

// Normalise puts a book in standard form, filling in what was left out.
func (b Book) Normalise() Book {
	b.Series = NormaliseSeries(b.Series)
	b.Label = strings.TrimSpace(b.Label)
	if b.SeqWidth < 1 || b.SeqWidth > SeqWidthMax {
		b.SeqWidth = SeqWidthDefault
	}
	if b.StartAt < 1 {
		b.StartAt = 1
	}
	return b
}

// Name is what the book is called on screen: its label where it has one, else
// its series, else the fact that it is the unlabelled one.
func (b Book) Name() string {
	switch {
	case b.Label != "":
		return b.Label
	case b.Series != "":
		return b.Series
	default:
		return "unlabelled series"
	}
}
