package consignment

import (
	"strings"
	"testing"
)

func TestDefaultRatesAreTheShippedConstants(t *testing.T) {
	got := DefaultRates()
	if got.RecipientReverseCharge != RateRecipientReverseChargeDefault {
		t.Errorf("reverse-charge rate = %s, want %s", got.RecipientReverseCharge, RateRecipientReverseChargeDefault)
	}
	if got.GTAForwardNoITC != RateGTAForwardNoITCDefault {
		t.Errorf("forward rate without credit = %s, want %s", got.GTAForwardNoITC, RateGTAForwardNoITCDefault)
	}
	if got.GTAForwardWithITC != RateGTAForwardWithITCDefault {
		t.Errorf("forward rate with credit = %s, want %s", got.GTAForwardWithITC, RateGTAForwardWithITCDefault)
	}
	if got.ApplyConsignmentExemption {
		t.Error("the exemption is applied by default; it must not be")
	}
	if got.ExemptSingleCarriageGross != ExemptSingleCarriageGrossDefault {
		t.Errorf("exemption figure = %s, want %s", got.ExemptSingleCarriageGross, ExemptSingleCarriageGrossDefault)
	}
}

func TestRatesNormalise(t *testing.T) {
	cases := []struct {
		name string
		in   Rates
		want Rates
	}{
		{
			name: "nothing set falls back to the shipped figures, not to a rate of nothing",
			in:   Rates{},
			want: DefaultRates(),
		},
		{
			name: "a field missing from the file falls back on its own",
			in:   Rates{RecipientReverseCharge: 250, GTAForwardNoITC: 250},
			want: Rates{
				RecipientReverseCharge:    250,
				GTAForwardNoITC:           250,
				GTAForwardWithITC:         RateGTAForwardWithITCDefault,
				ExemptSingleCarriageGross: ExemptSingleCarriageGrossDefault,
			},
		},
		{
			name: "an override is kept",
			in:   Rates{RecipientReverseCharge: 250, GTAForwardNoITC: 250, GTAForwardWithITC: 1800, ExemptSingleCarriageGross: 200000},
			want: Rates{RecipientReverseCharge: 250, GTAForwardNoITC: 250, GTAForwardWithITC: 1800, ExemptSingleCarriageGross: 200000},
		},
		{
			name: "the switch is kept as it was set",
			in:   Rates{ApplyConsignmentExemption: true},
			want: Rates{
				RecipientReverseCharge:    RateRecipientReverseChargeDefault,
				GTAForwardNoITC:           RateGTAForwardNoITCDefault,
				GTAForwardWithITC:         RateGTAForwardWithITCDefault,
				ApplyConsignmentExemption: true,
				ExemptSingleCarriageGross: ExemptSingleCarriageGrossDefault,
			},
		},
		{
			name: "a rate over a hundred percent falls back",
			in:   Rates{RecipientReverseCharge: PctScale + 1},
			want: DefaultRates(),
		},
		{
			name: "a negative rate falls back",
			in:   Rates{RecipientReverseCharge: -1, GTAForwardNoITC: -1, GTAForwardWithITC: -1, ExemptSingleCarriageGross: -1},
			want: DefaultRates(),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalise()
			if got != c.want {
				t.Errorf("Normalise() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestPositionsValidAndNamed(t *testing.T) {
	for _, p := range Positions() {
		if !p.Valid() {
			t.Errorf("%s is in Positions() but not Valid()", p)
		}
		if p.Name() == string(p) {
			t.Errorf("%s has no name of its own", p)
		}
	}
	if Position("whatever").Valid() {
		t.Error("an unknown position reported itself valid")
	}
	if PositionUnstated.Stated() {
		t.Error("the unstated position reported itself stated")
	}
	if !PositionRecipientReverseCharge.ReverseCharge() {
		t.Error("the reverse-charge position did not report itself as one")
	}
	if PositionGTAForwardWithITC.ReverseCharge() {
		t.Error("a forward-charge position reported itself as reverse charge")
	}
}

func TestTaxOnAnUnstatedPositionProducesNoFigure(t *testing.T) {
	n := carried()
	n.Tax = TaxStated{Position: PositionUnstated}

	got := TaxOn(n, DefaultRates())
	if got.Settled {
		t.Error("an unstated position reported itself settled")
	}
	if got.Amount != 0 {
		t.Errorf("amount = ₹%s, want nothing: the tool must not guess a position", got.Amount.Format())
	}
	if got.Rate != 0 {
		t.Errorf("rate = %s, want nothing", got.Rate)
	}
	if !strings.Contains(got.Endorsement, "NOT STATED") {
		t.Errorf("endorsement = %q, want it to say the position was not stated", got.Endorsement)
	}
	if !strings.Contains(got.Statement, "will not decide") {
		t.Errorf("statement = %q, want it to say plainly that the tool does not decide", got.Statement)
	}
}

func TestTaxOnEachStatedPosition(t *testing.T) {
	cases := []struct {
		name              string
		position          Position
		reason            string
		wantRate          Pct
		wantAmount        Paise // on a gross of ₹10,446.00
		wantInEndorsement string
	}{
		{
			name: "the recipient under reverse charge", position: PositionRecipientReverseCharge,
			wantRate: RateRecipientReverseChargeDefault, wantAmount: 52230,
			wantInEndorsement: "RECIPIENT",
		},
		{
			name: "the agency under forward charge, without credit", position: PositionGTAForwardNoITC,
			wantRate: RateGTAForwardNoITCDefault, wantAmount: 52230,
			wantInEndorsement: "FORWARD CHARGE",
		},
		{
			name: "the agency under forward charge, with credit", position: PositionGTAForwardWithITC,
			wantRate: RateGTAForwardWithITCDefault, wantAmount: 125352,
			wantInEndorsement: "FORWARD CHARGE",
		},
		{
			name: "no tax charged, with a reason", position: PositionNoTaxCharged,
			reason:   "recipient is not a specified recipient and no forward-charge election is in force",
			wantRate: 0, wantAmount: 0,
			wantInEndorsement: "NO TAX CHARGED",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			n.Tax = TaxStated{Position: c.position, Reason: c.reason}

			totals := Total(n)
			if want := Paise(1044600); totals.GrossPaise != want {
				t.Fatalf("the test's base changed: gross = ₹%s, want ₹%s", totals.GrossPaise.Format(), want.Format())
			}

			got := TaxOn(n, DefaultRates())
			if !got.Settled {
				t.Fatal("a stated position reported itself unsettled")
			}
			if got.Base != totals.GrossPaise {
				t.Errorf("base = ₹%s, want the gross freight ₹%s", got.Base.Format(), totals.GrossPaise.Format())
			}
			if got.Rate != c.wantRate {
				t.Errorf("rate = %s, want %s", got.Rate, c.wantRate)
			}
			if got.Amount != c.wantAmount {
				t.Errorf("amount = ₹%s, want ₹%s", got.Amount.Format(), c.wantAmount.Format())
			}
			if !strings.Contains(got.Endorsement, c.wantInEndorsement) {
				t.Errorf("endorsement = %q, want it to contain %q", got.Endorsement, c.wantInEndorsement)
			}
			if len(got.Caveats) == 0 {
				t.Error("a figure came back with no caveats attached")
			}
		})
	}
}

func TestEveryTaxViewCarriesTheTwoStandingCaveats(t *testing.T) {
	// A figure of this kind travelling without its caveats is the thing the
	// tool exists to prevent, so every view carries both: that the rates are
	// indicative and overridable, and that the position depends on the agency's
	// own election and the recipient's status.
	for _, position := range Positions() {
		n := carried()
		n.Tax = TaxStated{Position: position, Reason: "a reason"}
		got := TaxOn(n, DefaultRates())

		joined := strings.Join(got.Caveats, "\n")
		for _, want := range []string{
			"indicative",
			"can be changed on screen",
			"election",
			"status of the recipient",
			"does not decide",
			"separate document",
			"government portal",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("%s: caveats do not mention %q:\n%s", position, want, joined)
			}
		}
	}
}

func TestTaxRateOverrideOnOneNote(t *testing.T) {
	n := carried()
	n.Tax = TaxStated{Position: PositionRecipientReverseCharge, RateOverride: 250} // 2.5%

	got := TaxOn(n, DefaultRates())
	if got.Rate != 250 {
		t.Errorf("rate = %s, want the note's own 2.5%%", got.Rate)
	}
	if !strings.Contains(got.RateFrom, "entered on this note") {
		t.Errorf("rate came from %q, want it to say the note", got.RateFrom)
	}
	// 2.5% of ₹10,446.00
	if want := Paise(26115); got.Amount != want {
		t.Errorf("amount = ₹%s, want ₹%s", got.Amount.Format(), want.Format())
	}
	if !strings.Contains(strings.Join(got.Caveats, "\n"), "not the register's figure") {
		t.Error("the override was not called out in the caveats")
	}
}

func TestTaxRespondsToAChangedRegisterRate(t *testing.T) {
	// The rates live in the data file so that a change to the law is a change
	// to a number on screen and not a change to the program.
	n := carried()
	n.Tax = TaxStated{Position: PositionGTAForwardWithITC}

	rates := DefaultRates()
	rates.GTAForwardWithITC = 1800 // 18%

	got := TaxOn(n, rates)
	if got.Rate != 1800 {
		t.Errorf("rate = %s, want the register's changed 18%%", got.Rate)
	}
	if !strings.Contains(got.RateFrom, "register") {
		t.Errorf("rate came from %q, want it to say the register", got.RateFrom)
	}
	if want := Paise(188028); got.Amount != want {
		t.Errorf("amount = ₹%s, want ₹%s", got.Amount.Format(), want.Format())
	}
}

func TestTaxExemptionIsReportedButNotAppliedUnlessSwitchedOn(t *testing.T) {
	// A small consignment: a gross of ₹1,200, under the ₹1,500 figure.
	n := carried()
	n.Goods = []Goods{{Description: "A small parcel", Packages: 1, ActualGrams: 20 * GramsPerKilogram}}
	n.Charges = Charges{Basis: BasisLumpsum, FreightOverridden: true, FreightOverridePaise: 1200 * PaisePerRupee}
	n.Tax = TaxStated{Position: PositionRecipientReverseCharge}

	off := TaxOn(n, DefaultRates())
	if !off.ExemptionConsidered {
		t.Error("the exemption figure was not compared at all")
	}
	if !off.ExemptionWouldApply {
		t.Error("the gross is under the figure but the tool did not say so")
	}
	if off.ExemptionApplied {
		t.Error("the exemption was applied without being switched on")
	}
	if off.Amount == 0 {
		t.Error("no tax was worked out even though no exemption was applied")
	}
	if !strings.Contains(strings.Join(off.Caveats, "\n"), "not set to apply it") {
		t.Errorf("caveats do not explain that the exemption was not taken:\n%s", strings.Join(off.Caveats, "\n"))
	}

	rates := DefaultRates()
	rates.ApplyConsignmentExemption = true
	on := TaxOn(n, rates)
	if !on.ExemptionApplied {
		t.Error("the exemption was not applied even once switched on")
	}
	if on.Amount != 0 {
		t.Errorf("amount = ₹%s, want nothing once the exemption applies", on.Amount.Format())
	}
	if !strings.Contains(strings.Join(on.Caveats, "\n"), "has not checked that the exemption is in force") {
		t.Error("an applied exemption came back without the caution that the tool has not checked it")
	}
}

func TestTaxExemptionBoundary(t *testing.T) {
	cases := []struct {
		name       string
		gross      Paise
		wouldApply bool
	}{
		{"a rupee under the figure", ExemptSingleCarriageGrossDefault - 100, true},
		{"exactly the figure", ExemptSingleCarriageGrossDefault, true},
		{"a rupee over the figure", ExemptSingleCarriageGrossDefault + 100, false},
		{"nothing at all is not an exempt consignment, it is an empty one", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			n.Charges = Charges{Basis: BasisLumpsum, FreightOverridden: true, FreightOverridePaise: c.gross}
			n.Tax = TaxStated{Position: PositionRecipientReverseCharge}

			got := TaxOn(n, DefaultRates())
			if got.ExemptionWouldApply != c.wouldApply {
				t.Errorf("on a gross of ₹%s the exemption would apply = %v, want %v",
					c.gross.Format(), got.ExemptionWouldApply, c.wouldApply)
			}
		})
	}
}

func TestTaxNamesTheRecipientFromTheFreightTerm(t *testing.T) {
	cases := []struct {
		name       string
		terms      Terms
		wantGSTIN  string
		registered bool
	}{
		{"paid — the consignor", Terms{Basis: TermsPaid}, "29AAACS1234J1ZF", true},
		{"to pay — the consignee", Terms{Basis: TermsToPay}, "29AAFCM4567K1ZW", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := carried()
			n.Consignor.GSTIN = "29AAACS1234J1ZF"
			n.Consignee.GSTIN = "29AAFCM4567K1ZW"
			n.Terms = c.terms
			n.Tax = TaxStated{Position: PositionRecipientReverseCharge}

			got := TaxOn(n, DefaultRates())
			if got.RecipientGSTIN != c.wantGSTIN {
				t.Errorf("recipient's number = %q, want %q", got.RecipientGSTIN, c.wantGSTIN)
			}
			if got.RecipientRegistered != c.registered {
				t.Errorf("registered = %v, want %v", got.RecipientRegistered, c.registered)
			}
		})
	}
}

func TestTaxWarnsWhenTheRecipientIsUnidentifiedUnderReverseCharge(t *testing.T) {
	n := carried()
	n.Consignor.GSTIN = ""
	n.Terms = Terms{Basis: TermsPaid}
	n.Tax = TaxStated{Position: PositionRecipientReverseCharge}

	got := TaxOn(n, DefaultRates())
	if got.RecipientRegistered {
		t.Error("the recipient reported registered with no number recorded")
	}
	if !strings.Contains(strings.Join(got.Caveats, "\n"), "No registration number is recorded") {
		t.Errorf("caveats do not raise the missing number:\n%s", strings.Join(got.Caveats, "\n"))
	}
}

func TestTaxOnNoTaxChargedWithNoReason(t *testing.T) {
	n := carried()
	n.Tax = TaxStated{Position: PositionNoTaxCharged}

	got := TaxOn(n, DefaultRates())
	if !strings.Contains(got.Statement, "no reason given") {
		t.Errorf("statement = %q, want it to point out that no reason was given", got.Statement)
	}
}

func TestTaxStatedNormalise(t *testing.T) {
	got := TaxStated{
		Position:       Position("whatever"),
		Reason:         "  a reason  ",
		RecipientClass: "  a company  ",
		RateOverride:   -5,
	}.Normalise()

	if got.Position != PositionUnstated {
		t.Errorf("position = %s, want it to fall back to %s and not to a guess", got.Position, PositionUnstated)
	}
	if got.Reason != "a reason" || got.RecipientClass != "a company" {
		t.Errorf("fields were not trimmed: %+v", got)
	}
	if got.RateOverride != 0 {
		t.Errorf("rate override = %s, want a negative one cleared", got.RateOverride)
	}
	if over := (TaxStated{RateOverride: PctScale + 1}).Normalise(); over.RateOverride != 0 {
		t.Errorf("a rate over a hundred percent was kept: %s", over.RateOverride)
	}
}

func TestConstantsCarryTheirReferences(t *testing.T) {
	got := Constants(DefaultRates())
	if len(got) < 5 {
		t.Fatalf("only %d constants are published", len(got))
	}
	for _, c := range got {
		if c.Name == "" || c.Value == "" || c.What == "" || c.Reference == "" || c.Caution == "" {
			t.Errorf("%+v is published without one of its parts", c)
		}
	}

	// The three rates have to be there by name, so the screen and the explainer
	// cannot drift away from the code.
	byName := map[string]ConstantRef{}
	for _, c := range got {
		byName[c.Name] = c
	}
	for _, want := range []string{
		"RateRecipientReverseChargeDefault",
		"RateGTAForwardNoITCDefault",
		"RateGTAForwardWithITCDefault",
		"ExemptSingleCarriageGrossDefault",
	} {
		if _, ok := byName[want]; !ok {
			t.Errorf("%s is not published", want)
		}
	}
	if !strings.Contains(byName["RateRecipientReverseChargeDefault"].Caution, "Indicative") {
		t.Error("the reverse-charge rate is not labelled indicative")
	}
	if byName["ExemptSingleCarriageGrossDefault"].Applied {
		t.Error("the exemption is published as applied by default")
	}
	if !strings.Contains(byName["ExemptSingleCarriageGrossDefault"].Caution, "NOT applied") {
		t.Error("the exemption is not labelled as not applied")
	}
}

func TestConstantsFollowTheRegisterRates(t *testing.T) {
	rates := DefaultRates()
	rates.GTAForwardWithITC = 1800
	rates.ApplyConsignmentExemption = true

	for _, c := range Constants(rates) {
		if c.Name == "RateGTAForwardWithITCDefault" && c.Value != "18%" {
			t.Errorf("the published value is %q, want the register's 18%%", c.Value)
		}
		if c.Name == "ExemptSingleCarriageGrossDefault" && !c.Applied {
			t.Error("the exemption is published as not applied even though it is switched on")
		}
	}
}
