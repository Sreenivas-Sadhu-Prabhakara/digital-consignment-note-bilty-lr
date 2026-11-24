package consignment

import (
	"testing"
	"time"
)

func TestStatusValidAndNamed(t *testing.T) {
	for _, s := range Statuses() {
		if !s.Valid() {
			t.Errorf("%s is in Statuses() but not Valid()", s)
		}
		if s.Name() == string(s) {
			t.Errorf("%s has no name of its own", s)
		}
	}
	if Status("lost").Valid() {
		t.Error("an unknown status reported itself valid")
	}
}

func TestStatusGroups(t *testing.T) {
	cases := []struct {
		status   Status
		open     bool
		terminal bool
		carried  bool
	}{
		{StatusBooked, true, false, false},
		{StatusInTransit, true, false, true},
		{StatusArrived, true, false, true},
		{StatusDelivered, false, true, true},
		{StatusCancelled, false, true, false},
		{StatusVoid, false, true, false},
	}
	for _, c := range cases {
		t.Run(string(c.status), func(t *testing.T) {
			if got := c.status.Open(); got != c.open {
				t.Errorf("Open() = %v, want %v", got, c.open)
			}
			if got := c.status.Terminal(); got != c.terminal {
				t.Errorf("Terminal() = %v, want %v", got, c.terminal)
			}
			if got := c.status.Carried(); got != c.carried {
				t.Errorf("Carried() = %v, want %v", got, c.carried)
			}
		})
	}
}

func TestStatusStage(t *testing.T) {
	cases := []struct {
		status Status
		step   int
	}{
		{StatusBooked, 1},
		{StatusInTransit, 2},
		{StatusArrived, 3},
		{StatusDelivered, 4},
		{StatusCancelled, 0},
		{StatusVoid, 0},
	}
	for _, c := range cases {
		t.Run(string(c.status), func(t *testing.T) {
			step, of := c.status.Stage()
			if step != c.step {
				t.Errorf("step = %d, want %d", step, c.step)
			}
			if of != 4 {
				t.Errorf("of = %d, want 4", of)
			}
		})
	}
}

func TestAdvance(t *testing.T) {
	cases := []struct {
		name    string
		from    Status
		to      Status
		wantErr bool
	}{
		{"booked to in transit", StatusBooked, StatusInTransit, false},
		{"booked to arrived", StatusBooked, StatusArrived, false},
		{"booked straight to delivered for a local run", StatusBooked, StatusDelivered, false},
		{"booked to cancelled", StatusBooked, StatusCancelled, false},
		{"in transit to arrived", StatusInTransit, StatusArrived, false},
		{"in transit to delivered", StatusInTransit, StatusDelivered, false},
		{"in transit to cancelled", StatusInTransit, StatusCancelled, false},
		{"arrived to delivered", StatusArrived, StatusDelivered, false},
		{"arrived to cancelled", StatusArrived, StatusCancelled, false},
		{"staying where it is", StatusInTransit, StatusInTransit, false},

		{"in transit back to booked", StatusInTransit, StatusBooked, true},
		{"arrived back to in transit", StatusArrived, StatusInTransit, true},
		{"delivered back to anything", StatusDelivered, StatusArrived, true},
		{"delivered to cancelled", StatusDelivered, StatusCancelled, true},
		{"cancelled to booked", StatusCancelled, StatusBooked, true},
		{"void to booked", StatusVoid, StatusBooked, true},
		{"booked to void once something is on the number", StatusBooked, StatusVoid, true},
		{"in transit to void", StatusInTransit, StatusVoid, true},
		{"an unknown status to start from", Status("lost"), StatusDelivered, true},
		{"an unknown status to go to", StatusBooked, Status("lost"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Advance(c.from, c.to)
			if (err != nil) != c.wantErr {
				t.Errorf("Advance(%s, %s) = %v, want an error: %v", c.from, c.to, err, c.wantErr)
			}
		})
	}
}

func TestAdvanceAndNextAgree(t *testing.T) {
	// Next is what the screen offers; Advance is what the register enforces.
	// They have to be the same list or the screen offers a move that is refused.
	for _, from := range Statuses() {
		offered := map[Status]bool{}
		for _, to := range Next(from) {
			offered[to] = true
		}
		for _, to := range Statuses() {
			if to == from {
				continue
			}
			err := Advance(from, to)
			if offered[to] && err != nil {
				t.Errorf("Next(%s) offers %s but Advance refuses it: %v", from, to, err)
			}
			if !offered[to] && err == nil {
				t.Errorf("Advance(%s, %s) is allowed but Next(%s) does not offer it", from, to, from)
			}
		}
	}
}

func TestNextIsACopy(t *testing.T) {
	first := Next(StatusBooked)
	if len(first) == 0 {
		t.Fatal("nothing follows booked")
	}
	first[0] = Status("tampered")
	if Next(StatusBooked)[0] == Status("tampered") {
		t.Error("Next handed out the table itself")
	}
}

// deliverable is a note complete enough to be delivered.
func deliverable() Note {
	return Note{
		Number:    Number{Series: "BLR", Seq: 1},
		Date:      MustDate(2026, time.April, 6),
		Status:    StatusArrived,
		From:      "Bengaluru",
		To:        "Hubballi",
		Consignor: Party{Name: "A Consignor"},
		Consignee: Party{Name: "A Consignee"},
		Goods:     []Goods{{Description: "Bags of something", Packages: 10, ActualGrams: 500 * GramsPerKilogram}},
		Terms:     Terms{Basis: TermsPaid},
		Charges:   Charges{Basis: BasisPerQuintal, RatePaise: 100 * PaisePerRupee, AdvancePaise: 50000},
		Vehicle:   Vehicle{Number: "KA01AB1234"},
		Delivery: Delivery{
			Date:             MustDate(2026, time.April, 8),
			ReceiverName:     "A Receiver",
			PackagesReceived: 10,
			PackagesCounted:  true,
			Condition:        ConditionGoodOrder,
		},
	}
}

func TestCanSetChecksWhatTheNewStatusNeeds(t *testing.T) {
	cases := []struct {
		name    string
		note    func() Note
		to      Status
		wantErr bool
	}{
		{
			name: "a complete delivery record", note: deliverable,
			to: StatusDelivered, wantErr: false,
		},
		{
			name: "delivered with no delivery record",
			note: func() Note { n := deliverable(); n.Delivery = Delivery{}; return n },
			to:   StatusDelivered, wantErr: true,
		},
		{
			name: "delivered with no receiver named",
			note: func() Note { n := deliverable(); n.Delivery.ReceiverName = ""; return n },
			to:   StatusDelivered, wantErr: true,
		},
		{
			name: "cancelled with a reason and a date",
			note: func() Note {
				n := deliverable()
				n.Status = StatusBooked
				n.Cancellation = Cancellation{Date: MustDate(2026, time.April, 7), Reason: "consignor withdrew"}
				return n
			},
			to: StatusCancelled, wantErr: false,
		},
		{
			name: "cancelled with no reason",
			note: func() Note { n := deliverable(); n.Status = StatusBooked; return n },
			to:   StatusCancelled, wantErr: true,
		},
		{
			name: "cancelled with a reason but no date",
			note: func() Note {
				n := deliverable()
				n.Status = StatusBooked
				n.Cancellation = Cancellation{Reason: "consignor withdrew"}
				return n
			},
			to: StatusCancelled, wantErr: true,
		},
		{
			name: "cancelled before the note was issued",
			note: func() Note {
				n := deliverable()
				n.Status = StatusBooked
				n.Cancellation = Cancellation{Date: MustDate(2026, time.April, 1), Reason: "consignor withdrew"}
				return n
			},
			to: StatusCancelled, wantErr: true,
		},
		{
			name: "in transit with a vehicle",
			note: func() Note { n := deliverable(); n.Status = StatusBooked; return n },
			to:   StatusInTransit, wantErr: false,
		},
		{
			name: "in transit with no vehicle",
			note: func() Note {
				n := deliverable()
				n.Status = StatusBooked
				n.Vehicle = Vehicle{}
				return n
			},
			to: StatusInTransit, wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CanSet(c.note(), c.to)
			if (err != nil) != c.wantErr {
				t.Errorf("CanSet(..., %s) = %v, want an error: %v", c.to, err, c.wantErr)
			}
		})
	}
}

func TestCanSetVoidOnlyOnAnEmptyNumber(t *testing.T) {
	empty := Note{
		Number:       Number{Series: "BLR", Seq: 5},
		Date:         MustDate(2026, time.April, 11),
		Status:       StatusBooked,
		Cancellation: Cancellation{Date: MustDate(2026, time.April, 11), Reason: "leaf spoiled at the counter"},
	}
	// Void is reached from the status a fresh number sits at, and the status
	// table refuses the move from a booked consignment, so the register voids a
	// number that is already void rather than moving a live note into it.
	empty.Status = StatusVoid
	if err := CanSet(empty, StatusVoid); err != nil {
		t.Errorf("an empty number with a reason could not be voided: %v", err)
	}

	noReason := empty
	noReason.Cancellation.Reason = ""
	if err := CanSet(noReason, StatusVoid); err == nil {
		t.Error("a void number with no reason was allowed")
	}

	withGoods := empty
	withGoods.Consignor = Party{Name: "A Consignor"}
	withGoods.Goods = []Goods{{Description: "Something", Packages: 1}}
	if err := CanSet(withGoods, StatusVoid); err == nil {
		t.Error("a number with a consignment on it was allowed to be voided; it should be cancelled instead")
	}
}
