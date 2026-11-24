package consignment

import (
	"strings"
	"testing"
	"time"
)

func TestConditionsValidAndNamed(t *testing.T) {
	for _, c := range Conditions() {
		if !c.Valid() {
			t.Errorf("%s is in Conditions() but not Valid()", c)
		}
		if c.Name() == string(c) {
			t.Errorf("%s has no name of its own", c)
		}
	}
	if Condition("wet").Valid() {
		t.Error("an unknown condition reported itself valid")
	}
	if !ConditionGoodOrder.Clean() {
		t.Error("good order is not clean")
	}
	for _, c := range []Condition{ConditionShort, ConditionDamaged, ConditionShortAndDamaged, ConditionRefused} {
		if c.Clean() {
			t.Errorf("%s reported itself clean", c)
		}
	}
}

// booked is a note with 52 packages on it, delivered on 8 April.
func bookedWith52() Note {
	n := carried()
	n.Status = StatusDelivered
	return n
}

func TestCheckDelivery(t *testing.T) {
	cases := []struct {
		name     string
		delivery Delivery
		wantErr  string // a fragment of the error, or empty for no error
	}{
		{
			name: "a complete clean record",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
			},
		},
		{
			name:     "no date",
			delivery: Delivery{ReceiverName: "Shivanand Patil", PackagesReceived: 52, PackagesCounted: true},
			wantErr:  "the day the goods were taken",
		},
		{
			name: "no receiver named",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), PackagesReceived: 52, PackagesCounted: true,
			},
			wantErr: "name of the person who signed",
		},
		{
			name: "delivered before it was booked",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 1), ReceiverName: "Shivanand Patil",
				PackagesReceived: 52, PackagesCounted: true,
			},
			wantErr: "before they were booked",
		},
		{
			name: "delivered on the day it was booked",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 6), ReceiverName: "Shivanand Patil",
				PackagesReceived: 52, PackagesCounted: true,
			},
		},
		{
			name: "more packages than were carried",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 53, PackagesCounted: true,
			},
			wantErr: "cannot be more than was carried",
		},
		{
			name: "short with the condition still good order",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 50, PackagesCounted: true, Condition: ConditionGoodOrder,
			},
			wantErr: "cannot be good order",
		},
		{
			name: "short, said to be short, but with no remark",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 50, PackagesCounted: true, Condition: ConditionShort,
			},
			wantErr: "has to say what was wrong",
		},
		{
			name: "short, said to be short, with a remark",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 50, PackagesCounted: true, Condition: ConditionShort,
				Remarks: "Two bundles not on the vehicle at the destination.",
			},
		},
		{
			name: "damaged with no remark",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesReceived: 52, PackagesCounted: true, Condition: ConditionDamaged,
			},
			wantErr: "has to say what was wrong",
		},
		{
			name: "refused with no reason",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				Condition: ConditionRefused,
			},
			wantErr: "has to say what was wrong",
		},
		{
			name: "refused with a reason",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				Condition: ConditionRefused, Remarks: "Consignee closed; goods returned to the godown.",
			},
		},
		{
			name: "uncounted packages are allowed but not evidence",
			delivery: Delivery{
				Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
				PackagesCounted: false, Condition: ConditionGoodOrder,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckDelivery(bookedWith52(), c.delivery)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("CheckDelivery = %v, want no error", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckDelivery = nil, want an error about %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("CheckDelivery = %q, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestShortage(t *testing.T) {
	cases := []struct {
		name     string
		received int
		counted  bool
		want     int
	}{
		{"everything arrived", 52, true, 0},
		{"two short", 50, true, 2},
		{"nothing arrived", 0, true, 52},
		{"uncounted is not a shortage, it is an unknown", 0, false, 0},
		{"more than was carried is floored at nothing", 60, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Shortage(bookedWith52(), Delivery{PackagesReceived: c.received, PackagesCounted: c.counted})
			if got != c.want {
				t.Errorf("Shortage = %d, want %d", got, c.want)
			}
		})
	}
}

func TestDeliveryNormalise(t *testing.T) {
	got := Delivery{
		ReceiverName:          "  Shivanand Patil  ",
		ReceiverRole:          "  godown keeper  ",
		Remarks:               "  all well  ",
		IDShown:               "  employee card  ",
		Condition:             Condition("wet"),
		PackagesReceived:      -5,
		FreightCollectedPaise: -100,
	}.Normalise()

	if got.ReceiverName != "Shivanand Patil" || got.ReceiverRole != "godown keeper" ||
		got.Remarks != "all well" || got.IDShown != "employee card" {
		t.Errorf("fields were not trimmed: %+v", got)
	}
	if got.Condition != ConditionGoodOrder {
		t.Errorf("condition = %s, want it filled in as %s", got.Condition, ConditionGoodOrder)
	}
	if got.PackagesReceived != 0 || got.FreightCollectedPaise != 0 {
		t.Errorf("negatives were kept: %+v", got)
	}
}

func TestDeliveryRecorded(t *testing.T) {
	cases := []struct {
		name string
		in   Delivery
		want bool
	}{
		{"nothing", Delivery{}, false},
		{"a date", Delivery{Date: MustDate(2026, time.April, 8)}, true},
		{"a name", Delivery{ReceiverName: "Someone"}, true},
		{"a count", Delivery{PackagesCounted: true}, true},
		{"a remark", Delivery{Remarks: "something"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Recorded(); got != c.want {
				t.Errorf("Recorded() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestProofOfDeliveryOnACleanDelivery(t *testing.T) {
	n := bookedWith52()
	n.Delivery = Delivery{
		Date:                  MustDate(2026, time.April, 8),
		ReceiverName:          "Shivanand Patil",
		ReceiverRole:          "godown keeper, for Malnad Supplies",
		PackagesReceived:      52,
		PackagesCounted:       true,
		Condition:             ConditionGoodOrder,
		IDShown:               "employee card",
		FreightCollectedPaise: 0,
	}

	got := NewProofOfDelivery(n, 4)
	if !got.Clean {
		t.Error("a complete delivery of everything in good order is not clean")
	}
	if got.Shortage != 0 {
		t.Errorf("shortage = %d, want 0", got.Shortage)
	}
	if got.NoteNumber != "BLR/0001" {
		t.Errorf("note number = %q, want BLR/0001", got.NoteNumber)
	}
	if got.DeliveredLong != "8 April 2026" {
		t.Errorf("delivered on = %q, want it spelled out", got.DeliveredLong)
	}
	if got.DaysLate != 0 {
		t.Errorf("days late = %d, want none: two days' transit from 6 April lands on the 8th", got.DaysLate)
	}

	evidences := strings.Join(got.Evidences, "\n")
	for _, want := range []string{"Shivanand Patil", "8 April 2026", "52 of the 52 packages", "good order"} {
		if !strings.Contains(evidences, want) {
			t.Errorf("what it evidences does not mention %q:\n%s", want, evidences)
		}
	}

	// Even a clean one says what it does not evidence. The carrier did not open
	// the packages.
	notEvidences := strings.Join(got.DoesNotEvidence, "\n")
	if !strings.Contains(notEvidences, "did not open them") {
		t.Errorf("a clean proof of delivery does not disclaim the contents:\n%s", notEvidences)
	}
}

func TestProofOfDeliveryOnAShortDelivery(t *testing.T) {
	n := bookedWith52()
	n.Delivery = Delivery{
		Date:             MustDate(2026, time.April, 11),
		ReceiverName:     "Shivanand Patil",
		PackagesReceived: 50,
		PackagesCounted:  true,
		Condition:        ConditionShort,
		Remarks:          "Two bundles not on the vehicle at the destination.",
	}

	got := NewProofOfDelivery(n, 4)
	if got.Clean {
		t.Error("a short delivery reported itself clean")
	}
	if got.Shortage != 2 {
		t.Errorf("shortage = %d, want 2", got.Shortage)
	}
	// Expected on 8 April, delivered on the 11th.
	if got.DaysLate != 3 {
		t.Errorf("days late = %d, want 3", got.DaysLate)
	}
	if !strings.Contains(strings.Join(got.DoesNotEvidence, "\n"), "clean delivery") {
		t.Error("a short delivery does not say it is not a clean one")
	}
}

func TestProofOfDeliveryOnAnUncountedDelivery(t *testing.T) {
	n := bookedWith52()
	n.Delivery = Delivery{
		Date:         MustDate(2026, time.April, 8),
		ReceiverName: "Shivanand Patil",
		Condition:    ConditionGoodOrder,
	}

	got := NewProofOfDelivery(n, 4)
	if got.Clean {
		t.Error("an uncounted delivery reported itself clean")
	}
	if !strings.Contains(strings.Join(got.DoesNotEvidence, "\n"), "were not counted at delivery") {
		t.Errorf("it does not say the packages were never counted:\n%s", strings.Join(got.DoesNotEvidence, "\n"))
	}
}

func TestProofOfDeliveryOnAToPayNoteTracksTheMoney(t *testing.T) {
	n := bookedWith52()
	n.Terms = Terms{Basis: TermsToPay}
	totals := Total(n)
	n.Delivery = Delivery{
		Date:                  MustDate(2026, time.April, 8),
		ReceiverName:          "Shivanand Patil",
		PackagesReceived:      52,
		PackagesCounted:       true,
		Condition:             ConditionGoodOrder,
		FreightCollectedPaise: totals.BalancePaise - 50000, // ₹500 short
	}

	got := NewProofOfDelivery(n, 4)
	if want := Paise(50000); got.BalanceDue != want {
		t.Errorf("balance due = ₹%s, want ₹%s", got.BalanceDue.Format(), want.Format())
	}
	if !strings.Contains(strings.Join(got.Evidences, "\n"), "freight was collected") {
		t.Error("the collection is not evidenced")
	}

	paid := n
	paid.Delivery.FreightCollectedPaise = totals.BalancePaise
	if got := NewProofOfDelivery(paid, 4); got.BalanceDue != 0 {
		t.Errorf("balance due = ₹%s after collecting the whole balance", got.BalanceDue.Format())
	}

	over := n
	over.Delivery.FreightCollectedPaise = totals.BalancePaise + 100000
	if got := NewProofOfDelivery(over, 4); got.BalanceDue != 0 {
		t.Errorf("balance due = ₹%s after over-collecting; want it floored at nothing", got.BalanceDue.Format())
	}
}

func TestProofOfDeliveryOnANoteWithNoDeclaredValue(t *testing.T) {
	n := bookedWith52()
	n.DeclaredValuePaise = 0
	n.Delivery = Delivery{
		Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
		PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
	}

	got := NewProofOfDelivery(n, 4)
	if !strings.Contains(strings.Join(got.DoesNotEvidence, "\n"), "no value was declared") {
		t.Errorf("it does not say no value was declared:\n%s", strings.Join(got.DoesNotEvidence, "\n"))
	}
}

func TestProofOfDeliveryOnAnEmptyRecordSaysWhatIsMissing(t *testing.T) {
	n := bookedWith52()
	n.Delivery = Delivery{}

	got := NewProofOfDelivery(n, 4)
	if got.Clean {
		t.Error("an empty record reported itself clean")
	}
	if len(got.Evidences) != 0 {
		t.Errorf("an empty record evidences %v", got.Evidences)
	}
	if !strings.Contains(strings.Join(got.DoesNotEvidence, "\n"), "no receiver was named") {
		t.Errorf("it does not say nobody was named:\n%s", strings.Join(got.DoesNotEvidence, "\n"))
	}
}
