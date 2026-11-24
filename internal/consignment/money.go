// Package consignment holds the rules a road consignment note runs on: how it
// is numbered, whether the number series is intact, what the freight comes to
// and who owes it, what the tax position on the carriage is said to be, how the
// consignment moves from booked to delivered, and which particulars belong on
// which copy of the note.
//
// Everything here is a pure function over plain values. Nothing in this package
// reads a file, serves a request or talks to a network.
package consignment

import (
	"fmt"
	"strconv"
	"strings"
)

// Paise is money in whole paise — a hundredth of a rupee. Freight on a
// consignment note is added up, split between an advance and a balance, and
// used as the base a tax figure is worked out on, and every one of those has to
// come back to the same whole paise every time. A floating-point rupee does
// not, so there are none here.
type Paise int64

// Rupees is the major unit, used only where a figure is read as rupees.
const PaisePerRupee Paise = 100

// String writes the amount the way a consignment note prints it: grouped the
// Indian way, two places, no symbol.
func (p Paise) String() string { return p.Format() }

// Format writes the amount with Indian digit grouping — the last three digits
// together and pairs above that, so twelve lakh fifty thousand reads as
// 12,50,000.00 and not 1,250,000.00.
func (p Paise) Format() string {
	neg := p < 0
	v := p
	if neg {
		v = -v
	}

	whole := int64(v / PaisePerRupee)
	part := int64(v % PaisePerRupee)

	out := groupIndian(whole) + "." + fmt.Sprintf("%02d", part)
	if neg {
		return "-" + out
	}
	return out
}

// Rupees is the amount as whole rupees, truncated. Used where a figure is
// compared against a threshold stated in rupees.
func (p Paise) Rupees() int64 { return int64(p / PaisePerRupee) }

// groupIndian puts separators into a non-negative whole number the Indian way.
func groupIndian(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}

	head, tail := s[:len(s)-3], s[len(s)-3:]

	// Above the last three digits the digits go in pairs, read from the right.
	var parts []string
	for len(head) > 2 {
		parts = append([]string{head[len(head)-2:]}, parts...)
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append([]string{head}, parts...)
	}
	return strings.Join(parts, ",") + "," + tail
}

// ParsePaise reads an amount written as rupees — "1250", "1250.50", "₹1,250.5"
// — into whole paise. More than two decimal places is an error rather than a
// silent rounding, because a freight rate typed with three places is a typing
// mistake and not a precision the note can carry.
func ParsePaise(s string) (Paise, error) {
	t := strings.TrimSpace(s)
	t = strings.NewReplacer("₹", "", ",", "", " ", "").Replace(t)
	if t == "" {
		return 0, nil
	}

	neg := false
	switch t[0] {
	case '-':
		neg, t = true, t[1:]
	case '+':
		t = t[1:]
	}
	if t == "" {
		return 0, fmt.Errorf("%q is not an amount", s)
	}

	whole, frac, hasFrac := strings.Cut(t, ".")
	if whole == "" {
		whole = "0"
	}
	if hasFrac && len(frac) > 2 {
		return 0, fmt.Errorf("%q has more than two decimal places; an amount is in rupees and paise", s)
	}
	for frac != "" && len(frac) < 2 {
		frac += "0"
	}
	if frac == "" {
		frac = "00"
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", s)
	}

	out := Paise(w)*PaisePerRupee + Paise(f)
	if neg {
		out = -out
	}
	return out, nil
}

// Pct is a rate in hundredths of one percent, so five percent is 500 and
// twelve and a half percent is 1250. A tax rate has to be stated exactly and
// has to survive a change to a rate with a half in it, which a whole-number
// percentage cannot carry.
type Pct int64

// PctScale is what a Pct is measured in: 10,000 is one hundred percent.
const PctScale Pct = 10000

// String writes the rate the way a notification states it, with no more places
// than it needs: "5%", "12%", "2.5%".
func (p Pct) String() string {
	neg := p < 0
	v := p
	if neg {
		v = -v
	}

	whole := int64(v / 100)
	part := int64(v % 100)

	out := strconv.FormatInt(whole, 10)
	switch {
	case part == 0:
		// nothing
	case part%10 == 0:
		out += "." + strconv.FormatInt(part/10, 10)
	default:
		out += "." + fmt.Sprintf("%02d", part)
	}
	if neg {
		out = "-" + out
	}
	return out + "%"
}

// Of applies the rate to an amount, rounded once, half away from zero. A tax
// figure on a consignment note is rounded where it becomes a figure on the
// note and nowhere else.
func (p Pct) Of(base Paise) Paise {
	return Paise(divRound(int64(base)*int64(p), int64(PctScale)))
}

// ParsePct reads a rate written as a percentage — "5", "5%", "12.5" — into
// hundredths of a percent.
func ParsePct(s string) (Pct, error) {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if t == "" {
		return 0, nil
	}

	neg := false
	switch t[0] {
	case '-':
		neg, t = true, t[1:]
	case '+':
		t = t[1:]
	}
	if t == "" {
		return 0, fmt.Errorf("%q is not a rate", s)
	}

	whole, frac, hasFrac := strings.Cut(t, ".")
	if whole == "" {
		whole = "0"
	}
	if hasFrac && len(frac) > 2 {
		return 0, fmt.Errorf("%q is finer than a hundredth of a percent", s)
	}
	for frac != "" && len(frac) < 2 {
		frac += "0"
	}
	if frac == "" {
		frac = "00"
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a rate", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a rate", s)
	}

	out := Pct(w)*100 + Pct(f)
	if neg {
		out = -out
	}
	return out, nil
}

// divRound divides and rounds half away from zero, which is how a freight clerk
// rounds and what a consignee checking the balance on a note expects to find.
// A denominator of zero gives zero rather than a panic: a rate basis with no
// unit behind it is a validation finding, not a crash.
func divRound(num, den int64) int64 {
	if den == 0 {
		return 0
	}
	if (num < 0) != (den < 0) {
		return -divRoundPositive(abs64(num), abs64(den))
	}
	return divRoundPositive(abs64(num), abs64(den))
}

func divRoundPositive(num, den uint64) int64 {
	q := num / den
	if (num%den)*2 >= den {
		q++
	}
	return int64(q)
}

func abs64(v int64) uint64 {
	if v < 0 {
		return uint64(-v)
	}
	return uint64(v)
}
