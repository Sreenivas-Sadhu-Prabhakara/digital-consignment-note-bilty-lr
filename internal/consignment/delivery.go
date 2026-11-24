package consignment

import (
	"fmt"
	"strings"
)

// The delivery record, and the proof of delivery that comes out of it.
//
// A consignment note is discharged when somebody at the destination takes the
// goods and signs for them. What that signature is worth depends on what was
// written beside it: the day, the name of the person who signed, whether every
// package arrived, and whether anything was remarked on. A proof of delivery
// with a name and no date, or a short delivery with no remark, is the document
// a claim is later lost on.
//
// So the delivery record is not a free-text box. It is checked against the note
// it discharges, and the proof of delivery states plainly what it does and does
// not evidence.

// Delivery is the record of the goods being taken at the destination.
type Delivery struct {
	Date Date `json:"date"`
	// ReceiverName is who signed. "Received by" on a proof of delivery means a
	// person, not a company: a claim turns on who it was.
	ReceiverName string `json:"receiverName"`
	// ReceiverRole is how they come to be signing — the consignee, somebody for
	// the consignee, the security desk, a godown keeper.
	ReceiverRole string `json:"receiverRole"`
	// PackagesReceived is how many of the packages arrived. Zero with a
	// delivery date on the note means none were counted rather than none
	// arrived, which is why PackagesCounted says which it was.
	PackagesReceived int  `json:"packagesReceived"`
	PackagesCounted  bool `json:"packagesCounted"`

	// Condition is how the goods were when they were taken.
	Condition Condition `json:"condition"`
	Remarks   string    `json:"remarks"`

	// FreightCollectedPaise is what was collected at the destination, which on
	// a to-pay consignment is the balance.
	FreightCollectedPaise Paise `json:"freightCollectedPaise"`

	// IDShown is what the receiver produced to show they were entitled to the
	// goods, where anything was asked for.
	IDShown string `json:"idShown"`
}

// Condition is how the goods were when they were taken.
type Condition string

const (
	// ConditionGoodOrder is the ordinary case: everything arrived and nothing
	// was remarked on.
	ConditionGoodOrder Condition = "good-order"
	// ConditionShort is fewer packages than were booked.
	ConditionShort Condition = "short"
	// ConditionDamaged is packages arrived but damaged.
	ConditionDamaged Condition = "damaged"
	// ConditionShortAndDamaged is both.
	ConditionShortAndDamaged Condition = "short-and-damaged"
	// ConditionRefused is the receiver would not take the goods.
	ConditionRefused Condition = "refused"
)

// Conditions is every condition, in the order a delivery clerk meets them.
func Conditions() []Condition {
	return []Condition{ConditionGoodOrder, ConditionShort, ConditionDamaged, ConditionShortAndDamaged, ConditionRefused}
}

// Valid reports whether the condition is one this tool knows.
func (c Condition) Valid() bool {
	for _, known := range Conditions() {
		if c == known {
			return true
		}
	}
	return false
}

// Name is the condition in words, as it is printed on a proof of delivery.
func (c Condition) Name() string {
	switch c {
	case ConditionGoodOrder:
		return "Received in good order and condition"
	case ConditionShort:
		return "Received short"
	case ConditionDamaged:
		return "Received damaged"
	case ConditionShortAndDamaged:
		return "Received short and damaged"
	case ConditionRefused:
		return "Delivery refused"
	default:
		return string(c)
	}
}

// Clean reports whether the condition is the one that needs no remark.
func (c Condition) Clean() bool { return c == ConditionGoodOrder }

// Normalise trims the record and fills in a condition.
func (d Delivery) Normalise() Delivery {
	d.ReceiverName = strings.TrimSpace(d.ReceiverName)
	d.ReceiverRole = strings.TrimSpace(d.ReceiverRole)
	d.Remarks = strings.TrimSpace(d.Remarks)
	d.IDShown = strings.TrimSpace(d.IDShown)
	if !d.Condition.Valid() {
		d.Condition = ConditionGoodOrder
	}
	if d.PackagesReceived < 0 {
		d.PackagesReceived = 0
	}
	if d.FreightCollectedPaise < 0 {
		d.FreightCollectedPaise = 0
	}
	return d
}

// Recorded reports whether anything has been recorded at all.
func (d Delivery) Recorded() bool {
	return !d.Date.IsZero() || d.ReceiverName != "" || d.PackagesCounted || d.Remarks != ""
}

// CheckDelivery reports what is wrong with a delivery record against the note
// it discharges, or nothing where it is in order.
func CheckDelivery(n Note, d Delivery) error {
	n = n.Normalise()
	d = d.Normalise()

	if d.Date.IsZero() {
		return fmt.Errorf("a delivery record needs the day the goods were taken")
	}
	if d.ReceiverName == "" {
		return fmt.Errorf("a delivery record needs the name of the person who signed for the goods, not just the company's")
	}
	if !n.Date.IsZero() && d.Date.Before(n.Date) {
		return fmt.Errorf("the note is dated %s and the goods cannot have been delivered on %s, before they were booked", n.Date.Print(), d.Date.Print())
	}

	totals := Total(n)
	if d.PackagesCounted && d.PackagesReceived > totals.Packages {
		return fmt.Errorf("%d packages were booked and the record says %d arrived; a delivery cannot be more than was carried", totals.Packages, d.PackagesReceived)
	}

	short := d.PackagesCounted && d.PackagesReceived < totals.Packages
	if short && d.Condition.Clean() {
		return fmt.Errorf("%d of %d packages arrived, so the condition cannot be good order; say short, or short and damaged",
			d.PackagesReceived, totals.Packages)
	}
	if !d.Condition.Clean() && d.Remarks == "" {
		return fmt.Errorf("the condition is %q, so the record has to say what was wrong; a claim turns on this remark",
			strings.ToLower(d.Condition.Name()))
	}
	if d.Condition == ConditionRefused && d.Remarks == "" {
		return fmt.Errorf("a refused delivery has to say why it was refused")
	}
	return nil
}

// Shortage is how many packages did not arrive. It is zero where nothing was
// counted, because an uncounted delivery is not evidence of a complete one.
func Shortage(n Note, d Delivery) int {
	d = d.Normalise()
	if !d.PackagesCounted {
		return 0
	}
	short := Total(n).Packages - d.PackagesReceived
	if short < 0 {
		return 0
	}
	return short
}

// ProofOfDelivery is the delivery record as a document: what it evidences, and
// what it does not.
type ProofOfDelivery struct {
	NoteNumber string `json:"noteNumber"`
	NoteDate   Date   `json:"noteDate"`
	Route      string `json:"route"`

	Consignor string `json:"consignor"`
	Consignee string `json:"consignee"`

	DeliveredOn   Date   `json:"deliveredOn"`
	DeliveredLong string `json:"deliveredLong"`
	ReceivedBy    string `json:"receivedBy"`
	ReceiverRole  string `json:"receiverRole"`
	IDShown       string `json:"idShown"`

	PackagesBooked   int  `json:"packagesBooked"`
	PackagesReceived int  `json:"packagesReceived"`
	PackagesCounted  bool `json:"packagesCounted"`
	Shortage         int  `json:"shortage"`

	Condition        string `json:"condition"`
	ConditionClean   bool   `json:"conditionClean"`
	Remarks          string `json:"remarks"`
	TransitDays      int    `json:"transitDays"`
	ExpectedOn       Date   `json:"expectedOn"`
	DaysLate         int    `json:"daysLate"`
	FreightCollected Paise  `json:"freightCollected"`
	BalanceDue       Paise  `json:"balanceDue"`

	// Clean reports whether this is a clean proof of delivery — the goods were
	// taken, counted, in good order, and nothing was remarked on. Anything else
	// is a qualified one, and the difference matters to a claim.
	Clean bool `json:"clean"`
	// Evidences and DoesNotEvidence are printed on the document, because a
	// proof of delivery that overstates itself is worse than none.
	Evidences       []string `json:"evidences"`
	DoesNotEvidence []string `json:"doesNotEvidence"`
}

// NewProofOfDelivery builds the proof of delivery from a delivered note.
// It is built from whatever is recorded, including an incomplete record: the
// document then says what is missing rather than pretending to be clean.
func NewProofOfDelivery(n Note, width int) ProofOfDelivery {
	n = n.Normalise()
	d := n.Delivery
	totals := Total(n)

	out := ProofOfDelivery{
		NoteNumber:       n.Number.Format(width),
		NoteDate:         n.Date,
		Route:            n.Route(),
		Consignor:        n.Consignor.Name,
		Consignee:        n.Consignee.Name,
		DeliveredOn:      d.Date,
		DeliveredLong:    d.Date.Long(),
		ReceivedBy:       d.ReceiverName,
		ReceiverRole:     d.ReceiverRole,
		IDShown:          d.IDShown,
		PackagesBooked:   totals.Packages,
		PackagesReceived: d.PackagesReceived,
		PackagesCounted:  d.PackagesCounted,
		Shortage:         Shortage(n, d),
		Condition:        d.Condition.Name(),
		ConditionClean:   d.Condition.Clean(),
		Remarks:          d.Remarks,
		TransitDays:      n.TransitDays,
		ExpectedOn:       n.ExpectedDelivery(),
		FreightCollected: d.FreightCollectedPaise,
	}

	if expected := out.ExpectedOn; !expected.IsZero() && !d.Date.IsZero() {
		if late := expected.DaysUntil(d.Date); late > 0 {
			out.DaysLate = late
		}
	}

	out.BalanceDue = totals.BalancePaise - d.FreightCollectedPaise
	if out.BalanceDue < 0 {
		out.BalanceDue = 0
	}

	out.Clean = !d.Date.IsZero() && d.ReceiverName != "" && d.PackagesCounted &&
		out.Shortage == 0 && d.Condition.Clean()

	// What the document evidences, stated on the document.
	if !d.Date.IsZero() && d.ReceiverName != "" {
		out.Evidences = append(out.Evidences, fmt.Sprintf("that %s took delivery of this consignment on %s", d.ReceiverName, d.Date.Long()))
	}
	if d.PackagesCounted {
		out.Evidences = append(out.Evidences, fmt.Sprintf("that %d of the %d packages booked were received", d.PackagesReceived, totals.Packages))
	}
	if d.Condition.Clean() && out.Shortage == 0 && d.PackagesCounted {
		out.Evidences = append(out.Evidences, "that the goods were received in good order and condition so far as the receiver observed")
	}
	if d.FreightCollectedPaise > 0 {
		out.Evidences = append(out.Evidences, fmt.Sprintf("that ₹%s of freight was collected at the destination", d.FreightCollectedPaise.Format()))
	}

	// What it does not. These are the honest limits, and they go on the paper.
	out.DoesNotEvidence = append(out.DoesNotEvidence,
		"what was inside the packages: the carrier did not open them, and the goods are described as the consignor declared them")
	if !d.PackagesCounted {
		out.DoesNotEvidence = append(out.DoesNotEvidence,
			"that every package arrived: the packages were not counted at delivery, so this record is not evidence of a complete delivery")
	}
	if d.ReceiverName == "" {
		out.DoesNotEvidence = append(out.DoesNotEvidence,
			"who took the goods: no receiver was named")
	}
	if !d.Condition.Clean() {
		out.DoesNotEvidence = append(out.DoesNotEvidence,
			"a clean delivery: the condition was recorded as "+strings.ToLower(d.Condition.Name()))
	}
	if n.DeclaredValuePaise == 0 {
		out.DoesNotEvidence = append(out.DoesNotEvidence,
			"the value of the goods: no value was declared on the note")
	}
	return out
}
