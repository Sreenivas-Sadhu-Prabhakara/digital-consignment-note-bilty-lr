package consignment

import (
	"strings"
	"testing"
	"time"
)

// found reports whether a report has a finding of a severity against a field.
func found(r Report, sev Severity, field string) bool {
	for _, f := range r.Findings {
		if f.Severity == sev && f.Field == field {
			return true
		}
	}
	return false
}

// fields lists the findings of a severity, for a readable failure message.
func fields(r Report, sev Severity) string {
	var out []string
	for _, f := range r.Findings {
		if f.Severity == sev {
			out = append(out, f.Field+": "+f.What)
		}
	}
	return strings.Join(out, "\n")
}

// issuable is a note with no blockers on it.
func issuable() Note {
	return Note{
		Number:              Number{Series: "BLR", Seq: 1},
		Date:                MustDate(2026, time.April, 6),
		Status:              StatusBooked,
		From:                "Bengaluru",
		To:                  "Hubballi",
		Consignor:           Party{Name: "Sahyadri Hardware", GSTIN: "29AAACS1234J1ZF", PIN: "560053"},
		Consignee:           Party{Name: "Malnad Supplies", GSTIN: "29AAFCM4567K1ZW", PIN: "580030"},
		Goods:               []Goods{{Description: "Pipes", Packages: 40, Packing: "Bundles", ActualGrams: 4800 * GramsPerKilogram}},
		DeclaredValuePaise:  418000 * PaisePerRupee,
		ValueDeclaredByName: "Sahyadri Hardware",
		Terms:               Terms{Basis: TermsPaid},
		Charges: Charges{
			Basis:        BasisPerQuintal,
			RatePaise:    185 * PaisePerRupee,
			AdvancePaise: 888000,
		},
		Vehicle:        Vehicle{Number: "KA25C4471", Kind: "1109 / 10-tyre"},
		Driver:         Driver{Name: "Mahadev Kamble"},
		Tax:            TaxStated{Position: PositionRecipientReverseCharge},
		EWayBillNumber: "381004117762",
		TransitDays:    2,
	}
}

func TestCheckAnIssuableNoteHasNoBlockers(t *testing.T) {
	n := issuable()
	// Pay the gross exactly, so a paid note is really paid.
	n.Charges.AdvancePaise = Total(n).GrossPaise

	got := Check(n, DefaultRates(), testCarrier())
	if !got.Issuable {
		t.Fatalf("a complete note is not issuable:\n%s", fields(got, SeverityBlocker))
	}
	if got.Blockers != 0 {
		t.Errorf("blockers = %d:\n%s", got.Blockers, fields(got, SeverityBlocker))
	}
	if !strings.HasPrefix(got.Verdict, "Ready to issue") {
		t.Errorf("verdict = %q", got.Verdict)
	}
}

func TestCheckBlockers(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Note)
		field  string
	}{
		{"no number", func(n *Note) { n.Number.Seq = 0 }, "number"},
		{"a series that cannot be one", func(n *Note) { n.Number.Series = "BLR_X" }, "number.series"},
		{"no date", func(n *Note) { n.Date = Date{} }, "date"},
		{"no consignor", func(n *Note) { n.Consignor = Party{} }, "consignor.name"},
		{"no consignee", func(n *Note) { n.Consignee = Party{} }, "consignee.name"},
		{"no goods", func(n *Note) { n.Goods = nil }, "goods"},
		{"goods with no packages", func(n *Note) { n.Goods = []Goods{{Description: "Pipes", ActualGrams: 100}} }, "goods.packages"},
		{"a goods line with no description", func(n *Note) { n.Goods = []Goods{{Packages: 5, ActualGrams: 100}} }, "goods.0.description"},
		{"no place of origin", func(n *Note) { n.From = "" }, "from"},
		{"no place of delivery", func(n *Note) { n.To = "" }, "to"},
		{
			name:   "no vehicle on a consignment marked as carried",
			mutate: func(n *Note) { n.Status = StatusInTransit; n.Vehicle = Vehicle{} },
			field:  "vehicle.number",
		},
		{
			name: "to pay with no consignee",
			mutate: func(n *Note) {
				n.Terms = Terms{Basis: TermsToPay}
				n.Consignee = Party{}
			},
			field: "terms.basis",
		},
		{
			name: "to pay with no freight to collect",
			mutate: func(n *Note) {
				n.Terms = Terms{Basis: TermsToPay}
				n.Charges = Charges{Basis: BasisPerQuintal}
			},
			field: "charges",
		},
		{
			name:   "billed to a third party nobody named",
			mutate: func(n *Note) { n.Terms = Terms{Basis: TermsBilledToThird} },
			field:  "billTo.name",
		},
		{
			name: "charged per package with no packages",
			mutate: func(n *Note) {
				n.Charges.Basis = BasisPerPackage
				n.Goods = []Goods{{Description: "Loose grain", ActualGrams: 500 * GramsPerKilogram}}
			},
			field: "charges.basis",
		},
		{
			name: "no tax charged with no reason",
			mutate: func(n *Note) {
				n.Tax = TaxStated{Position: PositionNoTaxCharged}
			},
			field: "tax.reason",
		},
		{
			name: "delivered with no delivery record",
			mutate: func(n *Note) {
				n.Status = StatusDelivered
			},
			field: "delivery",
		},
		{
			name: "cancelled with no reason",
			mutate: func(n *Note) {
				n.Status = StatusCancelled
			},
			field: "cancellation.reason",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := issuable()
			n.Charges.AdvancePaise = Total(n).GrossPaise
			c.mutate(&n)

			got := Check(n, DefaultRates(), testCarrier())
			if !found(got, SeverityBlocker, c.field) {
				t.Errorf("no blocker on %q:\n%s", c.field, fields(got, SeverityBlocker))
			}
			if got.Issuable {
				t.Error("the note reported itself issuable with a blocker on it")
			}
			if !strings.HasPrefix(got.Verdict, "Not ready to issue") {
				t.Errorf("verdict = %q", got.Verdict)
			}
		})
	}
}

func TestCheckBlocksOnAMissingCarrier(t *testing.T) {
	n := issuable()
	n.Charges.AdvancePaise = Total(n).GrossPaise

	got := Check(n, DefaultRates(), Carrier{})
	if !found(got, SeverityBlocker, "carrier.name") {
		t.Errorf("a register with no carrier of its own is not blocked:\n%s", fields(got, SeverityBlocker))
	}
}

func TestCheckChecks(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Note)
		field  string
	}{
		{"no weight", func(n *Note) { n.Goods = []Goods{{Description: "Pipes", Packages: 40}} }, "goods.weight"},
		{"no vehicle yet on a booked note", func(n *Note) { n.Vehicle = Vehicle{} }, "vehicle.number"},
		{"a vehicle number of the wrong shape", func(n *Note) { n.Vehicle.Number = "XX" }, "vehicle.number"},
		{
			name:   "no driver on a consignment marked as carried",
			mutate: func(n *Note) { n.Status = StatusInTransit; n.Driver = Driver{} },
			field:  "driver.name",
		},
		{
			name:   "stamped paid but only part paid",
			mutate: func(n *Note) { n.Charges.AdvancePaise = 100 },
			field:  "charges.advancePaise",
		},
		{
			name:   "more advanced than the gross",
			mutate: func(n *Note) { n.Charges.AdvancePaise = 99999999 },
			field:  "charges.advancePaise",
		},
		{
			name:   "a value declared by nobody",
			mutate: func(n *Note) { n.ValueDeclaredByName = "" },
			field:  "valueDeclaredByName",
		},
		{
			name:   "a registration number that fails its check character",
			mutate: func(n *Note) { n.Consignee.GSTIN = "29AAFCM4567K1ZZ" },
			field:  "consignee.gstin",
		},
		{
			name:   "a postal index number of the wrong shape",
			mutate: func(n *Note) { n.Consignor.PIN = "5600" },
			field:  "consignor.pin",
		},
		{
			name: "reverse charge with an unidentified recipient",
			mutate: func(n *Note) {
				n.Tax = TaxStated{Position: PositionRecipientReverseCharge}
				n.Terms = Terms{Basis: TermsPaid}
				n.Consignor.GSTIN = ""
			},
			field: "tax.position",
		},
		{
			name: "a delivery record on a note that is not delivered",
			mutate: func(n *Note) {
				n.Delivery = Delivery{Date: MustDate(2026, time.April, 8), ReceiverName: "Someone"}
			},
			field: "delivery",
		},
		{
			name: "no freight at all",
			mutate: func(n *Note) {
				n.Charges = Charges{Basis: BasisPerQuintal}
			},
			field: "charges",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := issuable()
			n.Charges.AdvancePaise = Total(n).GrossPaise
			c.mutate(&n)

			got := Check(n, DefaultRates(), testCarrier())
			if !found(got, SeverityCheck, c.field) {
				t.Errorf("no check on %q:\n%s", c.field, fields(got, SeverityCheck))
			}
		})
	}
}

func TestCheckNotes(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Note)
		field  string
	}{
		{"no declared value", func(n *Note) { n.DeclaredValuePaise = 0 }, "declaredValuePaise"},
		{"no tax position", func(n *Note) { n.Tax = TaxStated{Position: PositionUnstated} }, "tax.position"},
		{"no e-way bill number", func(n *Note) { n.EWayBillNumber = "" }, "eWayBillNumber"},
		{"no transit time", func(n *Note) { n.TransitDays = 0 }, "transitDays"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := issuable()
			n.Charges.AdvancePaise = Total(n).GrossPaise
			c.mutate(&n)

			got := Check(n, DefaultRates(), testCarrier())
			if !found(got, SeverityNote, c.field) {
				t.Errorf("no note on %q:\n%s", c.field, fields(got, SeverityNote))
			}
			// None of these stops the note being issued.
			if !got.Issuable {
				t.Errorf("a note-level finding blocked the note:\n%s", fields(got, SeverityBlocker))
			}
		})
	}
}

func TestCheckOnADeliveredNote(t *testing.T) {
	base := func() Note {
		n := issuable()
		n.Charges.AdvancePaise = Total(n).GrossPaise
		n.Status = StatusDelivered
		n.Delivery = Delivery{
			Date: MustDate(2026, time.April, 8), ReceiverName: "Shivanand Patil",
			PackagesReceived: 40, PackagesCounted: true, Condition: ConditionGoodOrder,
		}
		return n
	}

	clean := Check(base(), DefaultRates(), testCarrier())
	if !clean.Issuable {
		t.Fatalf("a cleanly delivered note is blocked:\n%s", fields(clean, SeverityBlocker))
	}

	short := base()
	short.Delivery.PackagesReceived = 38
	short.Delivery.Condition = ConditionShort
	short.Delivery.Remarks = "Two bundles not on the vehicle."
	got := Check(short, DefaultRates(), testCarrier())
	if !found(got, SeverityCheck, "delivery.packagesReceived") {
		t.Errorf("a short delivery is not flagged:\n%s", fields(got, SeverityCheck))
	}

	uncounted := base()
	uncounted.Delivery.PackagesCounted = false
	uncounted.Delivery.PackagesReceived = 0
	got = Check(uncounted, DefaultRates(), testCarrier())
	if !found(got, SeverityCheck, "delivery.packagesCounted") {
		t.Errorf("an uncounted delivery is not flagged:\n%s", fields(got, SeverityCheck))
	}

	toPay := base()
	toPay.Terms = Terms{Basis: TermsToPay}
	toPay.Charges.AdvancePaise = 0
	toPay.Delivery.FreightCollectedPaise = 100
	got = Check(toPay, DefaultRates(), testCarrier())
	if !found(got, SeverityCheck, "delivery.freightCollectedPaise") {
		t.Errorf("goods released on a to-pay note without collecting the freight is not flagged:\n%s", fields(got, SeverityCheck))
	}
}

func TestCheckOnAVoidNumberChecksOnlyTheReason(t *testing.T) {
	void := Note{
		Number:       Number{Series: "BLR", Seq: 5},
		Date:         MustDate(2026, time.April, 11),
		Status:       StatusVoid,
		Cancellation: Cancellation{Date: MustDate(2026, time.April, 11), Reason: "leaf spoiled at the counter"},
	}
	got := Check(void, DefaultRates(), testCarrier())
	if !got.Issuable {
		t.Errorf("a properly accounted-for void number is blocked:\n%s", fields(got, SeverityBlocker))
	}
	if len(got.Findings) != 0 {
		t.Errorf("a void number was checked as a consignment:\n%s", fields(got, SeverityCheck)+fields(got, SeverityNote))
	}

	noReason := void
	noReason.Cancellation.Reason = ""
	got = Check(noReason, DefaultRates(), testCarrier())
	if !found(got, SeverityBlocker, "cancellation.reason") {
		t.Errorf("a void number with no reason is not blocked:\n%s", fields(got, SeverityBlocker))
	}
}

func TestCheckFlagsAnAppliedExemption(t *testing.T) {
	n := issuable()
	n.Charges = Charges{Basis: BasisLumpsum, FreightOverridden: true, FreightOverridePaise: 1200 * PaisePerRupee}
	n.Charges.AdvancePaise = 1200 * PaisePerRupee

	rates := DefaultRates()
	rates.ApplyConsignmentExemption = true

	got := Check(n, rates, testCarrier())
	if !found(got, SeverityCheck, "tax.position") {
		t.Errorf("an applied exemption is not flagged:\n%s", fields(got, SeverityCheck))
	}
}

func TestFindingsAreSortedWorstFirst(t *testing.T) {
	n := issuable()
	n.Consignee = Party{}        // a blocker
	n.Vehicle = Vehicle{}        // a check
	n.DeclaredValuePaise = 0     // a note
	n.Charges.AdvancePaise = 100 // another check

	got := Check(n, DefaultRates(), testCarrier())
	if got.Blockers == 0 || got.Checks == 0 || got.Notes == 0 {
		t.Fatalf("the test needs one of each: %d blockers, %d checks, %d notes", got.Blockers, got.Checks, got.Notes)
	}

	seen := map[Severity]bool{}
	last := ""
	for _, f := range got.Findings {
		switch f.Severity {
		case SeverityBlocker:
			if last != "" && last != string(SeverityBlocker) {
				t.Errorf("a blocker appears after a %s", last)
			}
		case SeverityCheck:
			if last == string(SeverityNote) {
				t.Error("a check appears after a note")
			}
		}
		seen[f.Severity] = true
		last = string(f.Severity)
	}
	if len(seen) != 3 {
		t.Errorf("not every severity came through: %v", seen)
	}
}

func TestEveryFindingSaysWhatToDo(t *testing.T) {
	// A finding with no remedy is a complaint. Every one has to say what to do
	// about it.
	for _, n := range append(SampleNotes(), Note{}, issuable()) {
		got := Check(n, DefaultRates(), testCarrier())
		for _, f := range got.Findings {
			if f.What == "" {
				t.Errorf("a finding on %q has nothing to say", f.Field)
			}
			if f.Fix == "" {
				t.Errorf("the finding %q on %q does not say what to do", f.What, f.Field)
			}
			if f.Field == "" {
				t.Errorf("the finding %q is not attached to a particular", f.What)
			}
		}
	}
}

func TestCheckOnAnEmptyNoteBlocksOnEverythingEssential(t *testing.T) {
	got := Check(Note{}, DefaultRates(), testCarrier())
	for _, want := range []string{"number", "date", "consignor.name", "consignee.name", "goods", "from", "to"} {
		if !found(got, SeverityBlocker, want) {
			t.Errorf("an empty note is not blocked on %q:\n%s", want, fields(got, SeverityBlocker))
		}
	}
	if got.Issuable {
		t.Error("an empty note reported itself issuable")
	}
}

func TestTheWorkedRegisterIsClean(t *testing.T) {
	// The worked register is what a user sees first, so a blocker in it would
	// teach them the wrong thing about their own notes.
	for _, n := range SampleNotes() {
		got := Check(n, SampleRates(), SampleCarrier())
		if !got.Issuable {
			t.Errorf("%s in the worked register is not issuable:\n%s",
				n.Number.Format(4), fields(got, SeverityBlocker))
		}
		// Nor should a worked note have its arithmetic queried. A note stamped
		// paid whose advance is short of its gross is a real finding, and a
		// sample that trips it is teaching the wrong thing by accident.
		if found(got, SeverityCheck, "charges.advancePaise") {
			t.Errorf("%s in the worked register has its advance queried:\n%s",
				n.Number.Format(4), fields(got, SeverityCheck))
		}
	}
}

func TestCancelledNotesAreNotAskedForwardLookingQuestions(t *testing.T) {
	// A cancelled note is not going anywhere. Asking for the vehicle it will
	// go on, or querying an advance against a carriage that never happened,
	// would bury the findings that matter under ones that cannot be answered.
	n := issuable()
	n.Status = StatusCancelled
	n.Cancellation = Cancellation{Date: n.Date, Reason: "consignor withdrew the booking"}
	n.Vehicle = Vehicle{}
	n.Charges.AdvancePaise = 0
	n.TransitDays = 0

	got := Check(n, DefaultRates(), testCarrier())
	for _, field := range []string{"vehicle.number", "charges.advancePaise"} {
		if found(got, SeverityCheck, field) {
			t.Errorf("a cancelled note is queried on %q:\n%s", field, fields(got, SeverityCheck))
		}
	}
	if !got.Issuable {
		t.Errorf("a properly cancelled note is blocked:\n%s", fields(got, SeverityBlocker))
	}

	// A live note with the same gaps is still queried on all of them.
	live := n
	live.Status = StatusBooked
	live.Cancellation = Cancellation{}
	liveReport := Check(live, DefaultRates(), testCarrier())
	for _, field := range []string{"vehicle.number", "charges.advancePaise"} {
		if !found(liveReport, SeverityCheck, field) {
			t.Errorf("a live note is not queried on %q:\n%s", field, fields(liveReport, SeverityCheck))
		}
	}
}
