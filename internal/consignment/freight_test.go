package consignment

import (
	"strings"
	"testing"
	"time"
)

// carried is a note with freight on it, used as the base for the freight and
// tax tests.
func carried() Note {
	return Note{
		Number:    Number{Series: "BLR", Seq: 1},
		Date:      MustDate(2026, time.April, 6),
		Status:    StatusInTransit,
		From:      "Bengaluru",
		To:        "Hubballi",
		Consignor: Party{Name: "Sahyadri Hardware"},
		Consignee: Party{Name: "Malnad Supplies"},
		Goods: []Goods{
			{Description: "Pipes", Packages: 40, Packing: "Bundles", ActualGrams: 4800 * GramsPerKilogram},
			{Description: "Fittings", Packages: 12, Packing: "Cartons", ActualGrams: 360 * GramsPerKilogram},
		},
		DeclaredValuePaise:  418000 * PaisePerRupee,
		ValueDeclaredByName: "Sahyadri Hardware",
		Terms:               Terms{Basis: TermsPaid},
		Charges: Charges{
			Basis:        BasisPerQuintal,
			RatePaise:    185 * PaisePerRupee,
			LoadingPaise: 900 * PaisePerRupee,
		},
		Vehicle:     Vehicle{Number: "KA25C4471"},
		Driver:      Driver{Name: "A Driver"},
		TransitDays: 2,
	}
}

func TestTermsValidAndStamped(t *testing.T) {
	for _, b := range TermsBases() {
		if !b.Valid() {
			t.Errorf("%s is in TermsBases() but not Valid()", b)
		}
		if b.Name() == string(b) {
			t.Errorf("%s has no name of its own", b)
		}
		if !strings.HasPrefix(b.Stamp(), "FREIGHT") {
			t.Errorf("%s stamps as %q", b, b.Stamp())
		}
	}
	if TermsBasis("maybe").Valid() {
		t.Error("an unknown term reported itself valid")
	}
	if got := TermsBasis("maybe").Stamp(); !strings.Contains(got, "NOT STATED") {
		t.Errorf("an unknown term stamps as %q, want it to say nothing was stated", got)
	}
}

func TestTermsNormaliseFillsInAnd(t *testing.T) {
	got := Terms{Basis: TermsBasis("maybe"), BilledToName: "  Someone  "}.Normalise()
	if got.Basis != TermsPaid {
		t.Errorf("basis = %s, want the default of %s", got.Basis, TermsPaid)
	}
	if got.BilledToName != "Someone" {
		t.Errorf("name = %q, want it trimmed", got.BilledToName)
	}
}

func TestTotal(t *testing.T) {
	n := carried()
	got := Total(n)

	if got.Packages != 52 {
		t.Errorf("packages = %d, want 52", got.Packages)
	}
	if got.GoodsLines != 2 {
		t.Errorf("goods lines = %d, want 2", got.GoodsLines)
	}
	if want := Grams(5160) * GramsPerKilogram; got.ActualGrams != want {
		t.Errorf("actual weight = %d, want %d", got.ActualGrams, want)
	}
	if got.ChargeableGrams != got.ActualGrams {
		t.Errorf("chargeable = %d, want the actual %d with no minimum set", got.ChargeableGrams, got.ActualGrams)
	}
	if got.MinimumApplied {
		t.Error("the minimum was applied with none set")
	}
	// 51.6 quintals × ₹185 = ₹9,546.00
	if want := Paise(954600); got.FreightPaise != want {
		t.Errorf("freight = %d (₹%s), want %d", got.FreightPaise, got.FreightPaise.Format(), want)
	}
	if !got.FreightFromRate {
		t.Error("freight did not come from the rate")
	}
	if want := Paise(954600 + 90000); got.GrossPaise != want {
		t.Errorf("gross = %d (₹%s), want %d", got.GrossPaise, got.GrossPaise.Format(), want)
	}
	if got.BalancePaise != got.GrossPaise {
		t.Errorf("balance = %d, want the whole gross with no advance", got.BalancePaise)
	}
	if got.GrossInWords == "" || !strings.HasPrefix(got.GrossInWords, "Rupees") {
		t.Errorf("gross in words = %q", got.GrossInWords)
	}
}

func TestTotalWithAMinimumChargedWeight(t *testing.T) {
	n := carried()
	n.Goods = []Goods{{Description: "Flanges", Packages: 8, ActualGrams: 1640 * GramsPerKilogram}}
	n.Charges.MinimumChargedGrams = 2000 * GramsPerKilogram
	n.Charges.RatePaise = 240 * PaisePerRupee

	got := Total(n)
	if !got.MinimumApplied {
		t.Error("the minimum was not applied even though it is above the load")
	}
	if want := Grams(2000) * GramsPerKilogram; got.ChargeableGrams != want {
		t.Errorf("chargeable = %d, want the minimum %d", got.ChargeableGrams, want)
	}
	// 20 quintals × ₹240 = ₹4,800
	if want := Paise(480000); got.FreightPaise != want {
		t.Errorf("freight = ₹%s, want ₹%s", got.FreightPaise.Format(), want.Format())
	}
}

func TestTotalWithAQuotedFreight(t *testing.T) {
	n := carried()
	n.Charges.FreightOverridden = true
	n.Charges.FreightOverridePaise = 18000 * PaisePerRupee

	got := Total(n)
	if got.FreightFromRate {
		t.Error("the freight came from the rate even though a figure was quoted")
	}
	if want := Paise(18000) * PaisePerRupee; got.FreightPaise != want {
		t.Errorf("freight = ₹%s, want the quoted ₹%s", got.FreightPaise.Format(), want.Format())
	}
}

func TestTotalOnAnEmptyNote(t *testing.T) {
	got := Total(Note{})
	if got.Packages != 0 || got.ActualGrams != 0 || got.GrossPaise != 0 {
		t.Errorf("an empty note totalled to %+v", got)
	}
	if got.GrossInWords != "Rupees Zero only" {
		t.Errorf("gross in words = %q, want it to spell zero out", got.GrossInWords)
	}
}

func TestNoteNormaliseDropsEmptyGoodsLines(t *testing.T) {
	n := Note{
		Goods: []Goods{
			{Description: "  Pipes  ", Packages: 40},
			{},
			{Description: "", Packages: 0, ActualGrams: 0, Packing: ""},
			{Packages: 5, ActualGrams: 100},
		},
		DeclaredValuePaise: -5,
		TransitDays:        -2,
	}
	got := n.Normalise()
	if len(got.Goods) != 2 {
		t.Fatalf("goods = %d lines, want 2 with the empty ones dropped", len(got.Goods))
	}
	if got.Goods[0].Description != "Pipes" {
		t.Errorf("description = %q, want it trimmed", got.Goods[0].Description)
	}
	if got.DeclaredValuePaise != 0 {
		t.Errorf("declared value = %d, want a negative one floored at zero", got.DeclaredValuePaise)
	}
	if got.TransitDays != 0 {
		t.Errorf("transit days = %d, want a negative one floored at zero", got.TransitDays)
	}
	if got.Status != StatusBooked {
		t.Errorf("status = %s, want it filled in as %s", got.Status, StatusBooked)
	}
}

func TestExpectedDelivery(t *testing.T) {
	cases := []struct {
		name  string
		date  Date
		days  int
		want  Date
		empty bool
	}{
		{"two days on", MustDate(2026, time.April, 6), 2, MustDate(2026, time.April, 8), false},
		{"across a month end", MustDate(2026, time.April, 29), 3, MustDate(2026, time.May, 2), false},
		{"across February in a common year", MustDate(2026, time.February, 27), 3, MustDate(2026, time.March, 2), false},
		{"across February in a leap year", MustDate(2024, time.February, 27), 3, MustDate(2024, time.March, 1), false},
		{"no transit time", MustDate(2026, time.April, 6), 0, Date{}, true},
		{"no date", Date{}, 2, Date{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			n.Date = c.date
			n.TransitDays = c.days
			got := n.ExpectedDelivery()
			if c.empty {
				if !got.IsZero() {
					t.Errorf("ExpectedDelivery() = %s, want nothing", got)
				}
				return
			}
			if got != c.want {
				t.Errorf("ExpectedDelivery() = %s, want %s", got, c.want)
			}
		})
	}
}

func TestLiableOnAPaidNote(t *testing.T) {
	n := carried()
	totals := Total(n)
	n.Charges.AdvancePaise = totals.GrossPaise

	got := Liable(n)
	if got.PayableBy != "Sahyadri Hardware" {
		t.Errorf("payable by = %q, want the consignor", got.PayableBy)
	}
	if got.CollectAtDelivery != 0 {
		t.Errorf("collect at delivery = %d, want nothing on a paid note", got.CollectAtDelivery)
	}
	if got.ReleaseAgainstPayment {
		t.Error("a paid note held the goods against payment")
	}
	if got.Outstanding != 0 {
		t.Errorf("outstanding = %d, want nothing once the gross was paid at booking", got.Outstanding)
	}
	if !strings.Contains(got.Explanation, "settled at booking") {
		t.Errorf("explanation = %q", got.Explanation)
	}
}

func TestLiableOnAPaidNoteThatWasNotActuallyPaid(t *testing.T) {
	// A note stamped paid whose advance is short is not paid. The carrier is
	// still owed the difference, and the register has to say so.
	n := carried()
	n.Charges.AdvancePaise = 5000 * PaisePerRupee

	totals := Total(n)
	got := Liable(n)
	want := totals.GrossPaise - n.Charges.AdvancePaise
	if got.Outstanding != want {
		t.Errorf("outstanding = ₹%s, want ₹%s", got.Outstanding.Format(), want.Format())
	}
	if !strings.Contains(got.Explanation, "still owed") {
		t.Errorf("explanation = %q, want it to say money is still owed", got.Explanation)
	}
}

func TestLiableOnAToPayNote(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsToPay}
	n.Charges.AdvancePaise = 5000 * PaisePerRupee

	totals := Total(n)
	got := Liable(n)
	if got.PayableBy != "Malnad Supplies" {
		t.Errorf("payable by = %q, want the consignee", got.PayableBy)
	}
	if got.CollectAtDelivery != totals.BalancePaise {
		t.Errorf("collect at delivery = ₹%s, want the balance ₹%s", got.CollectAtDelivery.Format(), totals.BalancePaise.Format())
	}
	if !got.ReleaseAgainstPayment {
		t.Error("a to-pay note released the goods without payment")
	}
	if !strings.Contains(got.Explanation, "advanced at booking") {
		t.Errorf("explanation = %q, want the advance mentioned", got.Explanation)
	}
}

func TestLiableOnAToPayNoteAlreadyCollected(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsToPay}
	n.Status = StatusDelivered

	totals := Total(n)
	n.Delivery = Delivery{
		Date:                  MustDate(2026, time.April, 8),
		ReceiverName:          "A Receiver",
		PackagesReceived:      52,
		PackagesCounted:       true,
		FreightCollectedPaise: totals.GrossPaise,
	}

	got := Liable(n)
	if got.Outstanding != 0 {
		t.Errorf("outstanding = ₹%s, want nothing once the freight was collected", got.Outstanding.Format())
	}
}

func TestLiableOnAThirdPartyNote(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsBilledToThird, BilledToName: "Konkan Procurement"}
	n.BillTo = Party{Name: "Konkan Procurement"}

	totals := Total(n)
	got := Liable(n)
	if got.PayableBy != "Konkan Procurement" {
		t.Errorf("payable by = %q, want the third party", got.PayableBy)
	}
	if got.CollectAtDelivery != 0 {
		t.Errorf("collect at delivery = %d, want nothing", got.CollectAtDelivery)
	}
	if got.ReleaseAgainstPayment {
		t.Error("a third-party note held the goods against payment")
	}
	if got.Outstanding != totals.GrossPaise {
		t.Errorf("outstanding = ₹%s, want the whole gross ₹%s", got.Outstanding.Format(), totals.GrossPaise.Format())
	}
}

func TestLiableFallsBackToTheRoleWhereNobodyIsNamed(t *testing.T) {
	n := carried()
	n.Terms = Terms{Basis: TermsToPay}
	n.Consignee = Party{}

	got := Liable(n)
	if !strings.Contains(got.Explanation, "the consignee") {
		t.Errorf("explanation = %q, want it to fall back to the role", got.Explanation)
	}
}

func TestLiableOwesNothingOnACancelledOrVoidNote(t *testing.T) {
	for _, status := range []Status{StatusCancelled, StatusVoid} {
		n := carried()
		n.Status = status
		n.Terms = Terms{Basis: TermsToPay}

		got := Liable(n)
		if got.Outstanding != 0 || got.CollectAtDelivery != 0 {
			t.Errorf("%s: outstanding ₹%s and collect ₹%s, want nothing of either",
				status, got.Outstanding.Format(), got.CollectAtDelivery.Format())
		}
		if !strings.Contains(got.Explanation, "no freight is outstanding") {
			t.Errorf("%s: explanation = %q", status, got.Explanation)
		}
	}
}

func TestRecipientFollowsTheFreightTerm(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Note)
		wantName string
	}{
		{"paid — the consignor", func(n *Note) { n.Terms = Terms{Basis: TermsPaid} }, "Sahyadri Hardware"},
		{"to pay — the consignee", func(n *Note) { n.Terms = Terms{Basis: TermsToPay} }, "Malnad Supplies"},
		{
			name: "billed to a third party — that party",
			mutate: func(n *Note) {
				n.Terms = Terms{Basis: TermsBilledToThird}
				n.BillTo = Party{Name: "Konkan Procurement"}
			},
			wantName: "Konkan Procurement",
		},
		{
			name: "billed to a third party named only on the term",
			mutate: func(n *Note) {
				n.Terms = Terms{Basis: TermsBilledToThird, BilledToName: "Konkan Procurement"}
			},
			wantName: "Konkan Procurement",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			c.mutate(&n)
			got, role := Recipient(n)
			if got.Name != c.wantName {
				t.Errorf("recipient = %q, want %q", got.Name, c.wantName)
			}
			if role == "" {
				t.Error("the recipient came back with no role")
			}
		})
	}
}

func TestRouteAndGoodsSummary(t *testing.T) {
	n := carried()
	if got, want := n.Route(), "Bengaluru to Hubballi"; got != want {
		t.Errorf("Route() = %q, want %q", got, want)
	}
	if got, want := n.GoodsSummary(), "Pipes, Fittings"; got != want {
		t.Errorf("GoodsSummary() = %q, want %q", got, want)
	}

	only := n
	only.To = ""
	if got, want := only.Route(), "from Bengaluru"; got != want {
		t.Errorf("Route() with no destination = %q, want %q", got, want)
	}
	only.From = ""
	only.To = "Hubballi"
	if got, want := only.Route(), "to Hubballi"; got != want {
		t.Errorf("Route() with no origin = %q, want %q", got, want)
	}
	if got := (Note{}).Route(); got != "" {
		t.Errorf("Route() on an empty note = %q, want empty", got)
	}

	many := n
	many.Goods = append(many.Goods,
		Goods{Description: "Clamps", Packages: 1},
		Goods{Description: "Brackets", Packages: 1},
	)
	if got, want := many.GoodsSummary(), "Pipes, Fittings and 2 other lines"; got != want {
		t.Errorf("GoodsSummary() with four lines = %q, want %q", got, want)
	}
	if got := (Note{}).GoodsSummary(); got != "" {
		t.Errorf("GoodsSummary() on an empty note = %q, want empty", got)
	}
}

func TestPartyLines(t *testing.T) {
	p := Party{
		Name:    "Sahyadri Hardware Stores",
		Address: "28, Chickpet Main Road\n\nSecond floor",
		City:    "Bengaluru",
		State:   "Karnataka",
		PIN:     "560053",
		GSTIN:   "29AAACS1234J1ZF",
		Phone:   "98450 11223",
	}
	got := p.Normalise().Lines()
	want := []string{
		"Sahyadri Hardware Stores",
		"28, Chickpet Main Road",
		"Second floor",
		"Bengaluru, Karnataka 560053",
		"GSTIN 29AAACS1234J1ZF",
		"Phone 98450 11223",
	}
	if len(got) != len(want) {
		t.Fatalf("Lines() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len((Party{}).Lines()) != 0 {
		t.Error("an empty party produced lines")
	}
	if (Party{}).Named() {
		t.Error("an empty party reported itself named")
	}
}
