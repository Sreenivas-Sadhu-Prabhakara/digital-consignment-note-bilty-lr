package consignment

import (
	"fmt"
	"strings"
)

// Where a consignment has got to, and what may follow what.
//
// The lifecycle is short because a road consignment's is: it is booked, it goes
// on a vehicle, it reaches the destination, and somebody signs for it. Two
// states sit outside that run and exist for the sake of the numbering: a note
// that was issued and then called off is cancelled, and a number that was never
// used at all is void. Both occupy their number for good. Nothing is deleted,
// because deleting a note would put a hole in the series.

// Status is where a consignment has got to.
type Status string

const (
	// StatusBooked is issued and accepted for carriage, not yet loaded out.
	StatusBooked Status = "booked"
	// StatusInTransit is on the vehicle and on the road.
	StatusInTransit Status = "in-transit"
	// StatusArrived is at the destination and waiting to be taken delivery of.
	StatusArrived Status = "arrived"
	// StatusDelivered is signed for by the receiver. Terminal.
	StatusDelivered Status = "delivered"
	// StatusCancelled is a note that was issued and whose consignment was then
	// called off. It keeps its number. Terminal.
	StatusCancelled Status = "cancelled"
	// StatusVoid is a number that was never used — a spoiled leaf in the book,
	// or a number skipped by mistake and accounted for afterwards. It carries
	// no consignment. Terminal.
	StatusVoid Status = "void"
)

// Statuses is every status, in the order a consignment meets them.
func Statuses() []Status {
	return []Status{StatusBooked, StatusInTransit, StatusArrived, StatusDelivered, StatusCancelled, StatusVoid}
}

// Valid reports whether the status is one this tool knows.
func (s Status) Valid() bool {
	for _, known := range Statuses() {
		if s == known {
			return true
		}
	}
	return false
}

// Name is the status in words, for the register and the copies.
func (s Status) Name() string {
	switch s {
	case StatusBooked:
		return "Booked"
	case StatusInTransit:
		return "In transit"
	case StatusArrived:
		return "Arrived at destination"
	case StatusDelivered:
		return "Delivered"
	case StatusCancelled:
		return "Cancelled"
	case StatusVoid:
		return "Void — number not used"
	default:
		return string(s)
	}
}

// Terminal reports whether nothing can follow this status.
func (s Status) Terminal() bool {
	switch s {
	case StatusDelivered, StatusCancelled, StatusVoid:
		return true
	default:
		return false
	}
}

// Open reports whether the consignment is still on the carrier's hands, which
// is what the register counts as outstanding.
func (s Status) Open() bool {
	switch s {
	case StatusBooked, StatusInTransit, StatusArrived:
		return true
	default:
		return false
	}
}

// Carried reports whether this status means the carriage happened — which is
// what decides whether the freight is owed at all.
func (s Status) Carried() bool {
	switch s {
	case StatusInTransit, StatusArrived, StatusDelivered:
		return true
	default:
		return false
	}
}

// allowed is what may follow what. Reaching the destination may be skipped,
// because a local delivery is booked and signed for the same afternoon and
// nobody records an arrival in between. Going backwards is not allowed: a
// consignment that was delivered was delivered.
var allowed = map[Status][]Status{
	StatusBooked:    {StatusInTransit, StatusArrived, StatusDelivered, StatusCancelled},
	StatusInTransit: {StatusArrived, StatusDelivered, StatusCancelled},
	StatusArrived:   {StatusDelivered, StatusCancelled},
	StatusDelivered: {},
	StatusCancelled: {},
	StatusVoid:      {},
}

// Next is what may follow a status.
func Next(from Status) []Status {
	out := allowed[from]
	copied := make([]Status, len(out))
	copy(copied, out)
	return copied
}

// Advance reports whether a consignment may move from one status to another,
// and says why not where it may not.
//
// It checks the move alone. Whether the note has what the new status needs — a
// delivery record for a delivery, a reason for a cancellation — is checked by
// CanSet, which is what the register calls.
func Advance(from, to Status) error {
	if !from.Valid() {
		return fmt.Errorf("%q is not a status this register keeps", from)
	}
	if !to.Valid() {
		return fmt.Errorf("%q is not a status this register keeps", to)
	}
	if from == to {
		return nil
	}
	if to == StatusVoid {
		return fmt.Errorf("a number can only be voided before anything is booked against it; this one is %s", from.Name())
	}
	for _, ok := range allowed[from] {
		if ok == to {
			return nil
		}
	}
	if from.Terminal() {
		return fmt.Errorf("this note is %s and nothing follows that", strings.ToLower(from.Name()))
	}
	return fmt.Errorf("a consignment cannot go from %s to %s", strings.ToLower(from.Name()), strings.ToLower(to.Name()))
}

// CanSet reports whether a note may be put into a status, checking both the
// move and whether the note carries what the new status needs. This is the one
// the register calls before it saves.
func CanSet(n Note, to Status) error {
	n = n.Normalise()
	if err := Advance(n.Status, to); err != nil {
		return err
	}

	switch to {
	case StatusDelivered:
		if err := CheckDelivery(n, n.Delivery); err != nil {
			return err
		}
	case StatusCancelled:
		if strings.TrimSpace(n.Cancellation.Reason) == "" {
			return fmt.Errorf("a cancelled note keeps its number, so the register has to say why it was cancelled")
		}
		if n.Cancellation.Date.IsZero() {
			return fmt.Errorf("say what day the consignment was called off")
		}
		if !n.Date.IsZero() && n.Cancellation.Date.Before(n.Date) {
			return fmt.Errorf("the note is dated %s and cannot have been cancelled on %s, before it was issued", n.Date.Print(), n.Cancellation.Date.Print())
		}
	case StatusVoid:
		if strings.TrimSpace(n.Cancellation.Reason) == "" {
			return fmt.Errorf("a void number has to say why it was never used — a spoiled leaf, or a number skipped and accounted for afterwards")
		}
		if n.Consignor.Named() || n.Consignee.Named() || len(n.Goods) > 0 {
			return fmt.Errorf("this number has a consignment on it; a number with goods against it is cancelled, not voided")
		}
	case StatusInTransit, StatusArrived:
		if n.Vehicle.Number == "" {
			return fmt.Errorf("say which vehicle the goods went on before marking the consignment %s", strings.ToLower(to.Name()))
		}
	}
	return nil
}

// Stage is how far along a consignment is, as a step out of a total, for a
// tracker on screen. A cancelled or void note is at no stage at all.
func (s Status) Stage() (step, of int) {
	switch s {
	case StatusBooked:
		return 1, 4
	case StatusInTransit:
		return 2, 4
	case StatusArrived:
		return 3, 4
	case StatusDelivered:
		return 4, 4
	default:
		return 0, 4
	}
}

// Cancellation is why a note was cancelled, or why a number was never used.
type Cancellation struct {
	Date   Date   `json:"date"`
	Reason string `json:"reason"`
	By     string `json:"by"`
}

// Normalise trims a cancellation.
func (c Cancellation) Normalise() Cancellation {
	c.Reason = strings.TrimSpace(c.Reason)
	c.By = strings.TrimSpace(c.By)
	return c
}
