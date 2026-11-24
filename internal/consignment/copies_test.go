package consignment

import (
	"strings"
	"testing"
	"time"
)

func testCarrier() Carrier {
	return Carrier{
		Name:    "Deccan Roadlines",
		Address: "Shed 14, Goods Transport Nagar",
		City:    "Bengaluru",
		State:   "Karnataka",
		PIN:     "560022",
		PAN:     "AABCD1234E",
		Phone:   "080 2345 6789",
	}
}

func TestCopyKindsValid(t *testing.T) {
	kinds := CopyKinds()
	if len(kinds) != 4 {
		t.Fatalf("got %d copies in the set, want four: three copies and the proof of delivery", len(kinds))
	}
	for _, k := range kinds {
		if !k.Valid() {
			t.Errorf("%s is in CopyKinds() but not Valid()", k)
		}
	}
	if CopyKind("pink").Valid() {
		t.Error("an unknown copy reported itself valid")
	}
}

func TestBuildWithholdsTheProofOfDeliveryUntilDelivery(t *testing.T) {
	cases := []struct {
		status    Status
		wantCount int
		withheld  bool
	}{
		{StatusBooked, 3, true},
		{StatusInTransit, 3, true},
		{StatusArrived, 3, true},
		{StatusDelivered, 4, false},
		{StatusCancelled, 3, true},
		{StatusVoid, 3, true},
	}
	for _, c := range cases {
		t.Run(string(c.status), func(t *testing.T) {
			n := carried()
			n.Status = c.status
			if c.status == StatusDelivered {
				n.Delivery = Delivery{
					Date: MustDate(2026, time.April, 8), ReceiverName: "A Receiver",
					PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
				}
			}

			got := Build(n, DefaultRates(), testCarrier(), 4)
			if len(got.Copies) != c.wantCount {
				t.Errorf("got %d copies, want %d", len(got.Copies), c.wantCount)
			}
			_, withheld := got.Withheld[string(CopyProofOfDelivery)]
			if withheld != c.withheld {
				t.Errorf("proof of delivery withheld = %v, want %v", withheld, c.withheld)
			}
			if c.withheld {
				if !strings.Contains(got.Withheld[string(CopyProofOfDelivery)], "Record the delivery first") {
					t.Errorf("the reason given is %q", got.Withheld[string(CopyProofOfDelivery)])
				}
			}
		})
	}
}

func TestEveryCopyNamesAllThreeParties(t *testing.T) {
	// A copy that does not name the carrier, the consignor and the consignee is
	// not evidence of a contract of carriage between them.
	n := carried()
	for _, kind := range []CopyKind{CopyConsignor, CopyConsignee, CopyCarrier} {
		t.Run(string(kind), func(t *testing.T) {
			got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
			if len(got.Parties) < 3 {
				t.Fatalf("got %d party blocks, want at least three", len(got.Parties))
			}
			labels := []string{got.Parties[0].Label, got.Parties[1].Label, got.Parties[2].Label}
			want := []string{"Carrier", "Consignor", "Consignee"}
			for i := range want {
				if labels[i] != want[i] {
					t.Errorf("party %d is labelled %q, want %q", i, labels[i], want[i])
				}
			}
			for i, block := range got.Parties[:3] {
				if len(block.Lines) == 0 {
					t.Errorf("party %d (%s) has no lines", i, block.Label)
				}
			}
		})
	}
}

func TestEveryCopyCarriesTheVehicleAndTheNumber(t *testing.T) {
	n := carried()
	for _, kind := range CopyKinds() {
		if kind == CopyProofOfDelivery {
			continue
		}
		got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
		if got.Number != "BLR/0001" {
			t.Errorf("%s: number = %q, want BLR/0001", kind, got.Number)
		}
		if got.Date != "06-04-2026" {
			t.Errorf("%s: date = %q", kind, got.Date)
		}
		if !strings.Contains(got.VehicleLine, "KA 25 C 4471") {
			t.Errorf("%s: vehicle = %q, want the registration number", kind, got.VehicleLine)
		}
		if got.Route != "Bengaluru to Hubballi" {
			t.Errorf("%s: route = %q", kind, got.Route)
		}
		if len(got.Goods) != 2 {
			t.Errorf("%s: got %d goods rows, want 2", kind, len(got.Goods))
		}
		if got.GoodsTotal.Packages != "52" {
			t.Errorf("%s: total packages = %q, want 52", kind, got.GoodsTotal.Packages)
		}
		if !strings.Contains(got.SaidTo, "has not opened the packages") {
			t.Errorf("%s: the copy does not say the goods are as declared: %q", kind, got.SaidTo)
		}
	}
}

func TestTheDriverIsOnlyOnTheCopiesThatNeedThem(t *testing.T) {
	n := carried()
	n.Driver = Driver{Name: "Mahadev Kamble", LicenceNo: "KA2520180004471", Phone: "99860 33445"}
	n.Status = StatusDelivered
	n.Delivery = Delivery{
		Date: MustDate(2026, time.April, 8), ReceiverName: "A Receiver",
		PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
	}

	cases := []struct {
		kind CopyKind
		want bool
	}{
		{CopyConsignor, false},
		{CopyConsignee, false},
		{CopyCarrier, true},
		{CopyProofOfDelivery, true},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			got := BuildCopy(n, DefaultRates(), testCarrier(), c.kind, 4)
			has := strings.Contains(got.DriverLine, "Mahadev Kamble")
			if has != c.want {
				t.Errorf("driver on the copy = %v (%q), want %v", has, got.DriverLine, c.want)
			}
			if c.want && !strings.Contains(got.DriverLine, "KA2520180004471") {
				t.Errorf("the licence number is not on the copy: %q", got.DriverLine)
			}
		})
	}
}

// moneyOn is every amount label and value on a copy, joined, for the freight
// disclosure tests.
func moneyOn(c Copy) string {
	var b strings.Builder
	for _, line := range c.Freight {
		b.WriteString(line.Label)
		b.WriteString(" = ")
		b.WriteString(line.Value)
		b.WriteString(" (")
		b.WriteString(line.Note)
		b.WriteString(")\n")
	}
	return b.String()
}

func TestAPrepaidConsigneeCopyShowsNoFreight(t *testing.T) {
	// The rate the consignor negotiated is not the consignee's business, and
	// nothing is collected at the destination, so the consignee's copy carries
	// no money at all beyond saying there is nothing to pay.
	n := carried()
	n.Terms = Terms{Basis: TermsPaid}
	totals := Total(n)
	n.Charges.AdvancePaise = totals.GrossPaise

	consignee := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignee, 4)
	money := moneyOn(consignee)

	if strings.Contains(money, totals.GrossPaise.Format()) {
		t.Errorf("the gross freight is on the consignee's prepaid copy:\n%s", money)
	}
	if strings.Contains(money, n.Charges.RatePaise.Format()) {
		t.Errorf("the rate is on the consignee's prepaid copy:\n%s", money)
	}
	if !strings.Contains(money, "Nil") {
		t.Errorf("the consignee's copy does not say there is nothing to pay:\n%s", money)
	}
	if consignee.AmountToCollect != "Nothing — freight paid" {
		t.Errorf("amount to collect = %q", consignee.AmountToCollect)
	}
	if !strings.Contains(strings.Join(consignee.Notes, "\n"), "does not appear on this copy") {
		t.Errorf("the copy does not explain why the freight is absent:\n%s", strings.Join(consignee.Notes, "\n"))
	}

	// The consignor's copy carries it in full.
	consignor := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignor, 4)
	if !strings.Contains(moneyOn(consignor), totals.GrossPaise.Format()) {
		t.Errorf("the consignor's own copy does not carry the gross freight:\n%s", moneyOn(consignor))
	}
}

func TestAToPayConsigneeCopyShowsWhatToPayButNotTheRate(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsToPay}
	liability := Liable(n)

	consignee := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignee, 4)
	money := moneyOn(consignee)

	if !strings.Contains(money, liability.CollectAtDelivery.Format()) {
		t.Errorf("the amount to pay is not on the consignee's to-pay copy:\n%s", money)
	}
	if !strings.Contains(money, "before the goods are released") {
		t.Errorf("the copy does not say the goods are held against payment:\n%s", money)
	}
	// Still not the rate it was worked out from.
	if strings.Contains(money, "185.00 qtl") || strings.Contains(money, "51.600 qtl") {
		t.Errorf("the rate or the working is on the consignee's copy:\n%s", money)
	}
	if consignee.FreightWords == "" {
		t.Error("the amount to pay is not stated in words on the consignee's copy")
	}
}

func TestAThirdPartyConsigneeCopySaysWhoIsBilled(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsBilledToThird, BilledToName: "Konkan Procurement"}
	n.BillTo = Party{Name: "Konkan Procurement", City: "Mangaluru"}

	consignee := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignee, 4)
	money := moneyOn(consignee)
	if !strings.Contains(money, "Konkan Procurement") {
		t.Errorf("the consignee's copy does not name who is billed:\n%s", money)
	}
	if consignee.AmountToCollect != "Nothing — freight billed to a third party" {
		t.Errorf("amount to collect = %q", consignee.AmountToCollect)
	}

	// The fourth party block appears on every copy of a third-party note.
	for _, kind := range []CopyKind{CopyConsignor, CopyConsignee, CopyCarrier} {
		got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
		found := false
		for _, block := range got.Parties {
			if block.Label == "Freight billed to" {
				found = true
				if len(block.Lines) == 0 {
					t.Errorf("%s: the billed-to block has no lines", kind)
				}
			}
		}
		if !found {
			t.Errorf("%s: no billed-to party block", kind)
		}
	}
}

func TestTheCarrierCopyCarriesTheWorkingAndABlankDeliveryBlock(t *testing.T) {
	n := carried()
	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyCarrier, 4)

	money := moneyOn(got)
	if !strings.Contains(money, "51.600 qtl") {
		t.Errorf("the carrier's copy does not show the units the freight was charged on:\n%s", money)
	}
	if !strings.Contains(money, "185.00") {
		t.Errorf("the carrier's copy does not show the rate:\n%s", money)
	}
	if !got.PODBlank {
		t.Error("the carrier's copy has no blank delivery block to be signed")
	}
	if len(got.Conditions) != len(Conditions()) {
		t.Errorf("got %d conditions to tick, want %d", len(got.Conditions), len(Conditions()))
	}
	if got.POD != nil {
		t.Error("the carrier's copy has a filled-in proof of delivery on it")
	}

	signatures := make([]string, 0, len(got.Signatures))
	for _, s := range got.Signatures {
		signatures = append(signatures, s.For)
	}
	joined := strings.Join(signatures, " | ")
	for _, want := range []string{"Goods booked by", "Driver", "Received by"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the carrier's copy has no %q signature block: %s", want, joined)
		}
	}
}

func TestTheMinimumWeightIsExplainedOnTheCopiesThatShowTheFreight(t *testing.T) {
	n := carried()
	n.Goods = []Goods{{Description: "Flanges", Packages: 8, ActualGrams: 1640 * GramsPerKilogram}}
	n.Charges.MinimumChargedGrams = 2000 * GramsPerKilogram

	for _, kind := range []CopyKind{CopyConsignor, CopyCarrier} {
		got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
		notes := strings.Join(got.Notes, "\n")
		if !strings.Contains(notes, "minimum weight") {
			t.Errorf("%s: the minimum is not explained:\n%s", kind, notes)
		}
	}

	consignee := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignee, 4)
	if strings.Contains(strings.Join(consignee.Notes, "\n"), "minimum weight") {
		t.Error("the minimum weight is explained on the consignee's copy, where the freight is not shown")
	}
}

func TestEveryCopyCarriesTheTaxEndorsement(t *testing.T) {
	cases := []struct {
		name     string
		position Position
		want     string
	}{
		{"unstated", PositionUnstated, "NOT STATED"},
		{"reverse charge", PositionRecipientReverseCharge, "RECIPIENT OF THE SERVICE UNDER REVERSE CHARGE"},
		{"forward charge", PositionGTAForwardWithITC, "GOODS TRANSPORT AGENCY UNDER FORWARD CHARGE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			n.Tax = TaxStated{Position: c.position}
			for _, kind := range []CopyKind{CopyConsignor, CopyConsignee, CopyCarrier} {
				got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
				if !strings.Contains(got.TaxEndorsement, c.want) {
					t.Errorf("%s: endorsement = %q, want it to contain %q", kind, got.TaxEndorsement, c.want)
				}
			}
		})
	}
}

func TestAnUnstatedTaxPositionPrintsNoFigure(t *testing.T) {
	n := carried()
	n.Tax = TaxStated{Position: PositionUnstated}

	for _, kind := range []CopyKind{CopyConsignor, CopyCarrier} {
		got := BuildCopy(n, DefaultRates(), testCarrier(), kind, 4)
		money := moneyOn(got)
		if !strings.Contains(money, "not stated") {
			t.Errorf("%s: the money block does not say the tax position is unstated:\n%s", kind, money)
		}
		if !strings.Contains(strings.Join(got.Notes, "\n"), "No tax position has been stated") {
			t.Errorf("%s: the notes do not say the position is unstated", kind)
		}
	}
}

func TestAReverseChargeFigureSaysItIsNotCollectedWithTheFreight(t *testing.T) {
	n := carried()
	n.Tax = TaxStated{Position: PositionRecipientReverseCharge}

	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignor, 4)
	money := moneyOn(got)
	if !strings.Contains(money, "not collected with this freight") {
		t.Errorf("a reverse-charge figure does not say it is not added to the note:\n%s", money)
	}
}

func TestACopySaysWhenNoEWayBillNumberIsRecorded(t *testing.T) {
	n := carried()
	n.EWayBillNumber = ""
	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignor, 4)
	notes := strings.Join(got.Notes, "\n")
	if !strings.Contains(notes, "separate document raised on the government portal") {
		t.Errorf("the copy does not say the e-way bill is separate:\n%s", notes)
	}

	with := carried()
	with.EWayBillNumber = "381004117762"
	withCopy := BuildCopy(with, DefaultRates(), testCarrier(), CopyConsignor, 4)
	found := false
	for _, f := range withCopy.Reference {
		if f.Label == "E-way bill no." {
			found = true
			if f.Value != "381004117762" {
				t.Errorf("the recorded number is %q", f.Value)
			}
			if !strings.Contains(f.Note, "government portal") {
				t.Errorf("the field does not say where the e-way bill comes from: %q", f.Note)
			}
		}
	}
	if !found {
		t.Error("the recorded e-way bill number is not on the copy")
	}
}

func TestACancelledCopySaysSoAndKeepsItsNumber(t *testing.T) {
	n := carried()
	n.Status = StatusCancelled
	n.Cancellation = Cancellation{Date: MustDate(2026, time.April, 7), Reason: "consignor withdrew the booking"}

	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyConsignor, 4)
	notes := strings.Join(got.Notes, "\n")
	if !strings.Contains(notes, "CANCELLED") {
		t.Errorf("a cancelled copy does not say so:\n%s", notes)
	}
	if !strings.Contains(notes, "number is retained and not reissued") {
		t.Errorf("a cancelled copy does not say the number is kept:\n%s", notes)
	}
	if got.Number != "BLR/0001" {
		t.Errorf("a cancelled copy lost its number: %q", got.Number)
	}
}

func TestAVoidCopySaysWhyTheNumberWasNeverUsed(t *testing.T) {
	n := Note{
		Number:       Number{Series: "BLR", Seq: 5},
		Date:         MustDate(2026, time.April, 11),
		Status:       StatusVoid,
		Cancellation: Cancellation{Date: MustDate(2026, time.April, 11), Reason: "leaf spoiled at the counter"},
	}
	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyCarrier, 4)
	notes := strings.Join(got.Notes, "\n")
	if !strings.Contains(notes, "VOID") || !strings.Contains(notes, "leaf spoiled") {
		t.Errorf("a void copy does not say why:\n%s", notes)
	}
	if !strings.Contains(notes, "series stays unbroken") {
		t.Errorf("a void copy does not say why it exists:\n%s", notes)
	}
}

func TestEachCopySaysWhatItIsAndWhatItIsFor(t *testing.T) {
	n := carried()
	n.Status = StatusDelivered
	n.Delivery = Delivery{
		Date: MustDate(2026, time.April, 8), ReceiverName: "A Receiver",
		PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
	}

	set := Build(n, DefaultRates(), testCarrier(), 4)
	seen := map[string]bool{}
	for _, copy := range set.Copies {
		if copy.Title == "" || copy.For == "" || copy.Purpose == "" {
			t.Errorf("%s is missing a title, a holder or a purpose: %+v", copy.Kind, copy)
		}
		if seen[copy.For] {
			t.Errorf("two copies claim to be %q", copy.For)
		}
		seen[copy.For] = true
		if copy.Foot == "" {
			t.Errorf("%s has no foot", copy.Kind)
		}
	}
	// The three copies say they are one of a set of three; the proof of
	// delivery does not, because it is not.
	for _, copy := range set.Copies {
		isOfSet := strings.Contains(copy.Foot, "one of a set of three")
		want := copy.Kind != CopyProofOfDelivery
		if isOfSet != want {
			t.Errorf("%s: foot says it is one of a set = %v, want %v (%q)", copy.Kind, isOfSet, want, copy.Foot)
		}
	}
}

func TestTheProofOfDeliveryCopyCarriesTheRecord(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsToPay}
	n.Status = StatusDelivered
	totals := Total(n)
	n.Delivery = Delivery{
		Date:                  MustDate(2026, time.April, 8),
		ReceiverName:          "Shivanand Patil",
		PackagesReceived:      52,
		PackagesCounted:       true,
		Condition:             ConditionGoodOrder,
		FreightCollectedPaise: totals.BalancePaise,
	}

	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyProofOfDelivery, 4)
	if got.POD == nil {
		t.Fatal("the proof of delivery has no record on it")
	}
	if got.POD.ReceivedBy != "Shivanand Patil" {
		t.Errorf("received by = %q", got.POD.ReceivedBy)
	}
	if !got.POD.Clean {
		t.Error("a complete delivery produced a qualified proof of delivery")
	}
	if got.PODBlank {
		t.Error("the proof of delivery is blank")
	}

	money := moneyOn(got)
	if !strings.Contains(money, "Collected") {
		t.Errorf("the proof of delivery does not show what was collected:\n%s", money)
	}
	if !strings.Contains(money, "Still due") {
		t.Errorf("the proof of delivery does not show what is left:\n%s", money)
	}
}

func TestTheCarrierBlockSaysWhenTheCarriersOwnDetailsAreMissing(t *testing.T) {
	n := carried()
	got := BuildCopy(n, DefaultRates(), Carrier{}, CopyConsignor, 4)
	if !strings.Contains(got.Parties[0].Note, "have not been entered") {
		t.Errorf("the carrier block does not say the details are missing: %q", got.Parties[0].Note)
	}
	if !strings.Contains(got.Foot, "the carrier") {
		t.Errorf("the foot does not fall back to a role: %q", got.Foot)
	}
}

func TestCarrierNormaliseAndParty(t *testing.T) {
	got := Carrier{
		Name: "  Deccan Roadlines  ", City: "  Bengaluru  ",
		GSTIN: " 29aaacs1234j1zf ", PAN: " aabcd1234e ", Terms: "  terms  ",
	}.Normalise()

	if got.Name != "Deccan Roadlines" || got.City != "Bengaluru" || got.Terms != "terms" {
		t.Errorf("fields were not trimmed: %+v", got)
	}
	if got.GSTIN != "29AAACS1234J1ZF" || got.PAN != "AABCD1234E" {
		t.Errorf("registration numbers were not normalised: %+v", got)
	}
	if lines := got.Party().Lines(); len(lines) == 0 || lines[0] != "Deccan Roadlines" {
		t.Errorf("Party().Lines() = %v", lines)
	}
}

func TestAnUnknownCopyKindFallsBackToTheConsignorCopy(t *testing.T) {
	got := BuildCopy(carried(), DefaultRates(), testCarrier(), CopyKind("pink"), 4)
	if got.Kind != CopyConsignor {
		t.Errorf("kind = %s, want it to fall back to %s", got.Kind, CopyConsignor)
	}
}

func TestVehicleAndDriverLinesFallBackWhenEmpty(t *testing.T) {
	n := carried()
	n.Vehicle = Vehicle{}
	n.Driver = Driver{}

	got := BuildCopy(n, DefaultRates(), testCarrier(), CopyCarrier, 4)
	if got.VehicleLine != "no vehicle recorded" {
		t.Errorf("vehicle line = %q", got.VehicleLine)
	}
	if got.DriverLine != "no driver recorded" {
		t.Errorf("driver line = %q", got.DriverLine)
	}
}

func TestTheWorkedRegisterProducesAFullSetForTheDeliveredNote(t *testing.T) {
	notes := SampleNotes()
	set := Build(notes[0], SampleRates(), SampleCarrier(), SampleBook().Normalise().SeqWidth)
	if len(set.Copies) != 4 {
		t.Fatalf("got %d copies for the delivered sample note, want all four", len(set.Copies))
	}
	if set.Number != "BLR/0001" {
		t.Errorf("number = %q, want BLR/0001", set.Number)
	}
}
