package consignment

import (
	"fmt"
	"strings"
)

// Who owes the freight, and when.
//
// A consignment note says one of three things about the freight, and which one
// it says governs the whole transaction. "Paid" means the consignor settled it
// at booking and the carrier delivers without collecting anything. "To pay"
// means the consignee settles it before the goods are released, which makes the
// note the carrier's only hold on the money. "Billed to" names a third party —
// a buying office, a principal, a clearing agent — who is invoiced separately,
// and the goods are released without collection at either end.
//
// The term is not cosmetic. It decides who the carrier can sue for the freight,
// whether the delivery clerk collects at the door, and which copy of the note
// shows what the carriage cost.

// Terms is the freight term on a consignment note.
type Terms struct {
	Basis TermsBasis `json:"basis"`
	// BilledToName is who the freight is billed to where the term is
	// billed-to-a-third-party, kept beside the full party details so a register
	// row can show it without carrying the whole block.
	BilledToName string `json:"billedToName"`
}

// TermsBasis is which of the three terms a note carries.
type TermsBasis string

const (
	// TermsPaid is freight settled by the consignor at booking.
	TermsPaid TermsBasis = "paid"
	// TermsToPay is freight collected from the consignee at the destination
	// before the goods are released.
	TermsToPay TermsBasis = "to-pay"
	// TermsBilledToThird is freight invoiced to a third party named on the
	// note, with nothing collected at either end.
	TermsBilledToThird TermsBasis = "billed-to-third-party"
)

// TermsBases is every term, in the order a booking clerk meets them.
func TermsBases() []TermsBasis { return []TermsBasis{TermsPaid, TermsToPay, TermsBilledToThird} }

// Valid reports whether the term is one this tool knows.
func (t TermsBasis) Valid() bool {
	for _, known := range TermsBases() {
		if t == known {
			return true
		}
	}
	return false
}

// Name is the term in words, for a label on screen.
func (t TermsBasis) Name() string {
	switch t {
	case TermsPaid:
		return "Paid — the consignor settled the freight at booking"
	case TermsToPay:
		return "To pay — the consignee settles before the goods are released"
	case TermsBilledToThird:
		return "Billed to a third party named on the note"
	default:
		return string(t)
	}
}

// Stamp is the two or three words a consignment note carries in the freight
// box, in the form the trade prints them.
func (t TermsBasis) Stamp() string {
	switch t {
	case TermsPaid:
		return "FREIGHT PAID"
	case TermsToPay:
		return "FREIGHT TO PAY"
	case TermsBilledToThird:
		return "FREIGHT BILLED TO THIRD PARTY"
	default:
		return "FREIGHT TERM NOT STATED"
	}
}

// Normalise fills in a term and trims the third party's name.
func (t Terms) Normalise() Terms {
	if !t.Basis.Valid() {
		t.Basis = TermsPaid
	}
	t.BilledToName = strings.TrimSpace(t.BilledToName)
	return t
}

// Liability is who owes the freight on this note, when, and how much.
type Liability struct {
	Basis TermsBasis `json:"basis"`
	Stamp string     `json:"stamp"`
	// PayableBy is the party that owes the freight, named.
	PayableBy string `json:"payableBy"`
	// PayableByRole says which party that is, so a copy can print "the
	// consignee" where the name is blank.
	PayableByRole string `json:"payableByRole"`
	// CollectAtDelivery is what the delivery clerk collects before releasing
	// the goods. Nothing, on a paid or third-party note.
	CollectAtDelivery Paise `json:"collectAtDelivery"`
	// SettledAtBooking is what was taken at booking.
	SettledAtBooking Paise `json:"settledAtBooking"`
	// Outstanding is what is still owed to the carrier on this consignment,
	// whoever owes it.
	Outstanding Paise `json:"outstanding"`
	// ReleaseAgainstPayment is whether the goods are held until the freight is
	// collected, which is the practical consequence of a to-pay note.
	ReleaseAgainstPayment bool `json:"releaseAgainstPayment"`
	// Explanation is the one line the note and the screen both print.
	Explanation string `json:"explanation"`
}

// Liable works out who owes the freight on a note and what is still outstanding
// against the delivery record.
func Liable(n Note) Liability {
	n = n.Normalise()
	totals := Total(n)

	out := Liability{
		Basis:            n.Terms.Basis,
		Stamp:            n.Terms.Basis.Stamp(),
		SettledAtBooking: totals.AdvancePaise,
	}

	switch n.Terms.Basis {
	case TermsPaid:
		out.PayableBy = n.Consignor.Name
		out.PayableByRole = "the consignor"
		out.CollectAtDelivery = 0
		out.ReleaseAgainstPayment = false
		// A note stamped paid whose advance is short of the gross is not paid.
		// The carrier is still owed the difference, from the consignor.
		out.Outstanding = totals.GrossPaise - totals.AdvancePaise
		if out.Outstanding < 0 {
			out.Outstanding = 0
		}
		if out.Outstanding > 0 {
			out.Explanation = fmt.Sprintf("Stamped paid, but ₹%s of the ₹%s gross freight was taken at booking. ₹%s is still owed by %s.",
				totals.AdvancePaise.Format(), totals.GrossPaise.Format(), out.Outstanding.Format(), orRole(out.PayableBy, out.PayableByRole))
		} else {
			out.Explanation = fmt.Sprintf("The freight of ₹%s was settled at booking by %s. Nothing is collected at delivery.",
				totals.GrossPaise.Format(), orRole(out.PayableBy, out.PayableByRole))
		}

	case TermsToPay:
		out.PayableBy = n.Consignee.Name
		out.PayableByRole = "the consignee"
		out.CollectAtDelivery = totals.BalancePaise
		if out.CollectAtDelivery < 0 {
			out.CollectAtDelivery = 0
		}
		out.ReleaseAgainstPayment = true
		out.Outstanding = out.CollectAtDelivery - n.Delivery.FreightCollectedPaise
		if out.Outstanding < 0 {
			out.Outstanding = 0
		}
		out.Explanation = fmt.Sprintf("₹%s is collected from %s before the goods are released.",
			out.CollectAtDelivery.Format(), orRole(out.PayableBy, out.PayableByRole))
		if totals.AdvancePaise > 0 {
			out.Explanation += fmt.Sprintf(" ₹%s of the ₹%s gross was advanced at booking.",
				totals.AdvancePaise.Format(), totals.GrossPaise.Format())
		}

	case TermsBilledToThird:
		out.PayableBy = firstNonEmpty(n.BillTo.Name, n.Terms.BilledToName)
		out.PayableByRole = "the party the freight is billed to"
		out.CollectAtDelivery = 0
		out.ReleaseAgainstPayment = false
		out.Outstanding = totals.GrossPaise - totals.AdvancePaise
		if out.Outstanding < 0 {
			out.Outstanding = 0
		}
		out.Explanation = fmt.Sprintf("₹%s is invoiced to %s. Nothing is collected at either end, and the goods are released without payment.",
			totals.GrossPaise.Format(), orRole(out.PayableBy, out.PayableByRole))
	}

	// A consignment that never moved owes no freight, whatever the note says.
	if n.Status == StatusCancelled || n.Status == StatusVoid {
		out.Outstanding = 0
		out.CollectAtDelivery = 0
		out.Explanation = fmt.Sprintf("This note is %s, so no freight is outstanding on it.", strings.ToLower(n.Status.Name()))
	}
	return out
}

// Recipient is who, on this note, is the recipient of the transport service —
// the person liable to pay the freight. It is that party, and not the consignee
// as such, whose own position decides how the tax on the carriage is accounted
// for, so the two are worked out from the same place.
func Recipient(n Note) (Party, string) {
	n = n.Normalise()
	switch n.Terms.Basis {
	case TermsToPay:
		return n.Consignee, "the consignee, who pays the freight"
	case TermsBilledToThird:
		if n.BillTo.Named() {
			return n.BillTo, "the party the freight is billed to"
		}
		return Party{Name: n.Terms.BilledToName}, "the party the freight is billed to"
	default:
		return n.Consignor, "the consignor, who paid the freight"
	}
}

func orRole(name, role string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return role
}

func firstNonEmpty(in ...string) string {
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}
