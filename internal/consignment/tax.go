package consignment

import (
	"fmt"
	"strings"
)

// The tax position on a road carriage, and why this tool will not decide it for
// you.
//
// Under the goods and services tax, a carrier who transports goods by road and
// issues a consignment note is a goods transport agency. The consignment note
// is the thing that makes it one: the definition in clause (ze) of paragraph 2
// of Notification No. 12/2017-Central Tax (Rate) turns on it. That is why this
// document, and not the lorry, is where the tax question starts.
//
// Where it goes next is not something a tool can settle. Tax on a goods
// transport agency's service is commonly payable by the recipient of the
// service under reverse charge, under Notification No. 13/2017-Central Tax
// (Rate) as amended, where the recipient is one of the categories that
// notification specifies. But the agency may instead elect to pay the tax
// itself under forward charge, and the rate then depends on which election it
// made and whether it takes input tax credit. Which of those applies to a given
// consignment therefore depends on two facts this tool cannot see: the agency's
// own election, and the status of the recipient of the service.
//
// So the tool does not guess. A note whose position has not been stated prints
// that it has not been stated. Once a position is chosen, the tool works out
// the figure that position implies, on the gross amount charged for the
// transportation of these goods, from a rate that is a named constant here, is
// labelled indicative, and can be changed on screen. Every figure it produces
// carries the caveats with it.
//
// Confirm the current rates, categories and exemptions with the Central Board
// of Indirect Taxes and Customs or your own adviser before you rely on any
// figure this tool prints. Notifications are amended, and what is written below
// is what this build ships with, not a statement of what is in force today.

// The rates this build ships with. Each is INDICATIVE and user-overridable:
// the register keeps its own copy in Rates, which is what the computation
// actually uses, and the screen lets it be changed.
const (
	// RateRecipientReverseChargeDefault is the rate commonly applied where the
	// recipient of a goods transport agency's service accounts for the tax
	// under reverse charge.
	//
	// Reference: Notification No. 13/2017-Central Tax (Rate) dated 28 June
	// 2017, as amended, read with Notification No. 11/2017-Central Tax (Rate)
	// dated 28 June 2017, as amended (transport of goods by road).
	// INDICATIVE — confirm the current rate and the specified categories of
	// recipient before relying on it.
	RateRecipientReverseChargeDefault Pct = 500 // 5.00%

	// RateGTAForwardNoITCDefault is the rate commonly applied where the agency
	// has elected to pay the tax itself and does not take input tax credit.
	//
	// Reference: Notification No. 11/2017-Central Tax (Rate) dated 28 June
	// 2017, as amended. INDICATIVE — confirm before relying on it.
	RateGTAForwardNoITCDefault Pct = 500 // 5.00%

	// RateGTAForwardWithITCDefault is the rate commonly applied where the
	// agency has elected to pay the tax itself and takes input tax credit.
	//
	// Reference: Notification No. 11/2017-Central Tax (Rate) dated 28 June
	// 2017, as amended. INDICATIVE — confirm before relying on it.
	RateGTAForwardWithITCDefault Pct = 1200 // 12.00%
)

// The small-consignment exemption figures this build carries. They are NOT
// applied by default, because the entries they came from have been amended and
// this tool will not assert that an exemption is available to you.
const (
	// ExemptSingleCarriageGrossDefault is the gross amount charged for the
	// transportation of goods in a single carriage below which an exemption was
	// historically available.
	//
	// Reference: Notification No. 12/2017-Central Tax (Rate) dated 28 June
	// 2017, entry against serial number 21 — since amended. INDICATIVE and NOT
	// APPLIED BY DEFAULT: check whether any such exemption is in force and
	// available to you before switching it on.
	ExemptSingleCarriageGrossDefault Paise = 150000 // ₹1,500.00

	// ExemptSingleConsigneeGrossDefault is the equivalent figure where the
	// goods for a single consignee are what is in question.
	//
	// Reference: as above, and subject to the same caution.
	ExemptSingleConsigneeGrossDefault Paise = 75000 // ₹750.00

	// ApplyConsignmentExemptionDefault is off. The tool does not apply an
	// exemption nobody told it applies.
	ApplyConsignmentExemptionDefault = false
)

// Rates is the register's own copy of the figures above — what the computation
// actually uses. It is kept in the data file so that a change to the law is a
// change to a number on screen and not a change to the program.
type Rates struct {
	RecipientReverseCharge Pct `json:"recipientReverseCharge"`
	GTAForwardNoITC        Pct `json:"gtaForwardNoItc"`
	GTAForwardWithITC      Pct `json:"gtaForwardWithItc"`

	ApplyConsignmentExemption bool  `json:"applyConsignmentExemption"`
	ExemptSingleCarriageGross Paise `json:"exemptSingleCarriageGross"`
}

// DefaultRates is the figures this build ships with.
func DefaultRates() Rates {
	return Rates{
		RecipientReverseCharge:    RateRecipientReverseChargeDefault,
		GTAForwardNoITC:           RateGTAForwardNoITCDefault,
		GTAForwardWithITC:         RateGTAForwardWithITCDefault,
		ApplyConsignmentExemption: ApplyConsignmentExemptionDefault,
		ExemptSingleCarriageGross: ExemptSingleCarriageGrossDefault,
	}
}

// Normalise fills in anything left out with the shipped figure, and refuses a
// rate that cannot be one.
//
// A field at zero counts as left out, not as a deliberate rate of nothing. That
// is the safe way round: a data file written by an older build, or edited by
// hand and missing a field, would otherwise come back with a rate of nought and
// work out no tax at all without saying so. A register-wide rate of zero is not
// a position anybody holds anyway — "no tax on this carriage" is a position
// chosen on the note, which states a reason with it, and a rate that has
// genuinely changed is entered as the rate it changed to.
func (r Rates) Normalise() Rates {
	d := DefaultRates()
	if r.RecipientReverseCharge <= 0 || r.RecipientReverseCharge > PctScale {
		r.RecipientReverseCharge = d.RecipientReverseCharge
	}
	if r.GTAForwardNoITC <= 0 || r.GTAForwardNoITC > PctScale {
		r.GTAForwardNoITC = d.GTAForwardNoITC
	}
	if r.GTAForwardWithITC <= 0 || r.GTAForwardWithITC > PctScale {
		r.GTAForwardWithITC = d.GTAForwardWithITC
	}
	if r.ExemptSingleCarriageGross <= 0 {
		r.ExemptSingleCarriageGross = d.ExemptSingleCarriageGross
	}
	return r
}

// Position is what the note says about the tax on this carriage.
type Position string

const (
	// PositionUnstated is the default and the honest one. The note says the
	// position has not been settled, and no figure is printed.
	PositionUnstated Position = "unstated"
	// PositionRecipientReverseCharge is the recipient of the service
	// accounting for the tax under reverse charge.
	PositionRecipientReverseCharge Position = "recipient-reverse-charge"
	// PositionGTAForwardNoITC is the agency paying the tax itself, without
	// input tax credit.
	PositionGTAForwardNoITC Position = "gta-forward-no-itc"
	// PositionGTAForwardWithITC is the agency paying the tax itself, with
	// input tax credit.
	PositionGTAForwardWithITC Position = "gta-forward-with-itc"
	// PositionNoTaxCharged is no tax charged on this consignment note, for a
	// reason the user states. The tool records the reason; it does not supply
	// one and does not confirm it.
	PositionNoTaxCharged Position = "no-tax-charged"
)

// Positions is every position, in the order they are offered.
func Positions() []Position {
	return []Position{
		PositionUnstated,
		PositionRecipientReverseCharge,
		PositionGTAForwardNoITC,
		PositionGTAForwardWithITC,
		PositionNoTaxCharged,
	}
}

// Valid reports whether the position is one this tool knows.
func (p Position) Valid() bool {
	for _, known := range Positions() {
		if p == known {
			return true
		}
	}
	return false
}

// Name is the position in words, as it is offered on screen.
func (p Position) Name() string {
	switch p {
	case PositionUnstated:
		return "Not settled yet — the note will say so rather than state a figure"
	case PositionRecipientReverseCharge:
		return "Payable by the recipient of the service under reverse charge"
	case PositionGTAForwardNoITC:
		return "Payable by this agency under forward charge, without input tax credit"
	case PositionGTAForwardWithITC:
		return "Payable by this agency under forward charge, with input tax credit"
	case PositionNoTaxCharged:
		return "No tax charged on this consignment note, for a stated reason"
	default:
		return string(p)
	}
}

// ReverseCharge reports whether this position puts the tax on the recipient.
func (p Position) ReverseCharge() bool { return p == PositionRecipientReverseCharge }

// Stated reports whether the position says anything at all.
func (p Position) Stated() bool { return p != PositionUnstated && p != "" }

// TaxStated is what the note says about the tax: the position, and whatever the
// user had to supply for it.
type TaxStated struct {
	Position Position `json:"position"`
	// Reason is why no tax is charged, where that is the position.
	Reason string `json:"reason"`
	// RecipientClass is how the recipient of the service describes itself,
	// recorded because whether the recipient falls in a specified category is
	// the fact the reverse-charge question turns on. It is free text on
	// purpose: this tool does not keep a list of categories and then imply that
	// picking one off it settles the law.
	RecipientClass string `json:"recipientClass"`
	// RateOverride, where set above zero, is used in place of the register's
	// rate for this note alone — for a consignment carried under a rate that
	// has since changed, or one the adviser says is different.
	RateOverride Pct `json:"rateOverride"`
}

// Normalise trims the statement and fills in a position.
func (t TaxStated) Normalise() TaxStated {
	if !t.Position.Valid() {
		t.Position = PositionUnstated
	}
	t.Reason = strings.TrimSpace(t.Reason)
	t.RecipientClass = strings.TrimSpace(t.RecipientClass)
	if t.RateOverride < 0 || t.RateOverride > PctScale {
		t.RateOverride = 0
	}
	return t
}

// TaxView is the tax position on one note, worked out — with the caveats
// attached, because a figure of this kind travelling without them is the thing
// the tool is trying to prevent.
type TaxView struct {
	Position Position `json:"position"`
	Settled  bool     `json:"settled"`

	// Base is the gross amount charged for the transportation of these goods —
	// the figure the rate is applied to.
	Base Paise `json:"base"`
	Rate Pct   `json:"rate"`
	// RateFrom says where the rate came from: the register's figure, or an
	// override on this note.
	RateFrom string `json:"rateFrom"`
	Amount   Paise  `json:"amount"`

	// PayableBy is who the chosen position says accounts for the tax.
	PayableBy     string `json:"payableBy"`
	PayableByName string `json:"payableByName"`

	// Endorsement is the line the consignment note carries about the tax. The
	// trade has long printed on a consignment note who is liable to pay the tax
	// on the carriage, and the practice carried over from the particulars
	// prescribed by rule 4B of the Service Tax Rules, 1994. Where no position
	// has been stated, the endorsement says exactly that.
	Endorsement string `json:"endorsement"`
	// Statement is the longer sentence shown on screen.
	Statement string `json:"statement"`

	// ExemptionConsidered is whether the small-consignment exemption figure was
	// compared at all, and ExemptionWouldApply whether the gross fell under it.
	// The figure is not applied unless the register was told to apply it.
	ExemptionConsidered  bool   `json:"exemptionConsidered"`
	ExemptionWouldApply  bool   `json:"exemptionWouldApply"`
	ExemptionApplied     bool   `json:"exemptionApplied"`
	ExemptionThreshold   Paise  `json:"exemptionThreshold"`
	RecipientRegistered  bool   `json:"recipientRegistered"`
	RecipientGSTIN       string `json:"recipientGstin"`
	RecipientDescription string `json:"recipientDescription"`

	// Caveats travel with the figure wherever it goes.
	Caveats []string `json:"caveats"`
}

// TaxOn works out the tax position on a note from what the note says and the
// register's rates. It never chooses a position: an unstated one stays unstated
// and produces no figure.
func TaxOn(n Note, rates Rates) TaxView {
	n = n.Normalise()
	rates = rates.Normalise()
	totals := Total(n)
	recipient, recipientRole := Recipient(n)

	out := TaxView{
		Position:             n.Tax.Position,
		Base:                 totals.GrossPaise,
		RecipientGSTIN:       recipient.GSTIN,
		RecipientRegistered:  recipient.GSTIN != "",
		RecipientDescription: recipientRole,
		ExemptionThreshold:   rates.ExemptSingleCarriageGross,
	}

	// The exemption figure is compared and reported, and applied only where the
	// register was told to apply it.
	if rates.ExemptSingleCarriageGross > 0 {
		out.ExemptionConsidered = true
		out.ExemptionWouldApply = totals.GrossPaise > 0 && totals.GrossPaise <= rates.ExemptSingleCarriageGross
		out.ExemptionApplied = rates.ApplyConsignmentExemption && out.ExemptionWouldApply
	}

	switch n.Tax.Position {
	case PositionRecipientReverseCharge:
		out.Settled = true
		out.Rate, out.RateFrom = pickRate(rates.RecipientReverseCharge, n.Tax.RateOverride)
		out.PayableBy = "the recipient of the service, under reverse charge"
		out.PayableByName = orRole(recipient.Name, recipientRole)
		out.Endorsement = "TAX ON THIS CARRIAGE IS PAYABLE BY THE RECIPIENT OF THE SERVICE UNDER REVERSE CHARGE"

	case PositionGTAForwardNoITC:
		out.Settled = true
		out.Rate, out.RateFrom = pickRate(rates.GTAForwardNoITC, n.Tax.RateOverride)
		out.PayableBy = "this agency, under forward charge, without input tax credit"
		out.PayableByName = "the goods transport agency issuing this note"
		out.Endorsement = "TAX ON THIS CARRIAGE IS PAYABLE BY THE GOODS TRANSPORT AGENCY UNDER FORWARD CHARGE"

	case PositionGTAForwardWithITC:
		out.Settled = true
		out.Rate, out.RateFrom = pickRate(rates.GTAForwardWithITC, n.Tax.RateOverride)
		out.PayableBy = "this agency, under forward charge, with input tax credit"
		out.PayableByName = "the goods transport agency issuing this note"
		out.Endorsement = "TAX ON THIS CARRIAGE IS PAYABLE BY THE GOODS TRANSPORT AGENCY UNDER FORWARD CHARGE"

	case PositionNoTaxCharged:
		out.Settled = true
		out.Rate, out.RateFrom = 0, "no rate: no tax is charged on this note"
		out.PayableBy = "nobody — no tax is charged on this consignment note"
		out.Endorsement = "NO TAX CHARGED ON THIS CONSIGNMENT NOTE"
		if n.Tax.Reason != "" {
			out.Endorsement += " — " + strings.ToUpper(n.Tax.Reason)
		}

	default:
		out.Settled = false
		out.Rate, out.RateFrom = 0, "no rate: the position has not been stated"
		out.PayableBy = "not stated on this note"
		out.Endorsement = "TAX POSITION ON THIS CARRIAGE NOT STATED ON THIS NOTE"
	}

	if out.Settled && out.Rate > 0 {
		if out.ExemptionApplied {
			out.Amount = 0
		} else {
			out.Amount = out.Rate.Of(out.Base)
		}
	}

	out.Statement = taxStatement(out, n)
	out.Caveats = taxCaveats(out, n, rates)
	return out
}

// pickRate takes the note's own rate where it has one, and says which was used.
func pickRate(registerRate, override Pct) (Pct, string) {
	if override > 0 {
		return override, "a rate entered on this note"
	}
	return registerRate, "the register's rate for this position"
}

func taxStatement(v TaxView, n Note) string {
	switch {
	case !v.Settled:
		return "The tax position on this carriage has not been stated. Whether the tax falls on the recipient under reverse charge or on this agency under forward charge depends on the election this agency has made and on the status of the recipient of the service, and this tool will not decide either. Choose a position, or the note will print that none was stated."
	case v.Position == PositionNoTaxCharged && n.Tax.Reason == "":
		return "This note says no tax is charged, with no reason given. State the reason: it is the reason, and not the note, that will have to stand up."
	case v.Position == PositionNoTaxCharged:
		return fmt.Sprintf("No tax is charged on this note: %s. The reason is recorded as entered and has not been checked against anything.", n.Tax.Reason)
	case v.ExemptionApplied:
		return fmt.Sprintf("The gross freight of ₹%s is at or below the exemption figure of ₹%s this register carries, and the register has been set to apply it, so no tax is worked out. Whether any such exemption is in force and available to you is for you to confirm.",
			v.Base.Format(), v.ExemptionThreshold.Format())
	case v.Amount == 0 && v.Base == 0:
		return "There is no freight on this note yet, so there is nothing for a rate to apply to."
	default:
		return fmt.Sprintf("At %s on the gross freight of ₹%s, the tax on this carriage comes to ₹%s, accounted for by %s. The rate used is %s.",
			v.Rate, v.Base.Format(), v.Amount.Format(), v.PayableBy, v.RateFrom)
	}
}

// taxCaveats is what has to travel with any figure this produces.
func taxCaveats(v TaxView, n Note, rates Rates) []string {
	out := []string{
		"The rates this tool carries are indicative and can be changed on screen. Confirm the current position with the Central Board of Indirect Taxes and Customs or your own adviser before relying on any figure here.",
		"Whether tax on this carriage falls on the recipient under reverse charge or on this agency under forward charge depends on the election this agency has made and on the status of the recipient of the service. This tool records what you tell it and works out the figure that follows; it does not decide which applies.",
	}

	if v.Position.ReverseCharge() {
		out = append(out, "Where the recipient accounts for the tax under reverse charge, this agency does not collect it with the freight. The figure above is what the recipient would account for, not an amount to add to this note.")
		if !v.RecipientRegistered {
			out = append(out, fmt.Sprintf("No registration number is recorded for %s. Whether a recipient falls in a specified category is the fact the reverse-charge question turns on, so record it.", v.RecipientDescription))
		}
	}
	if v.Position == PositionGTAForwardWithITC || v.Position == PositionGTAForwardNoITC {
		out = append(out, "Paying under forward charge is an election with conditions attached, including how and when it is made and whether input tax credit may be taken. Check that the election covers the period this note falls in.")
	}
	if v.ExemptionConsidered && v.ExemptionWouldApply && !v.ExemptionApplied {
		out = append(out, fmt.Sprintf("The gross freight of ₹%s is at or below the exemption figure of ₹%s this build carries, but the register is not set to apply it, so no exemption has been taken. The entries that figure came from have been amended; check whether any exemption is in force and available to you before switching it on.",
			v.Base.Format(), rates.ExemptSingleCarriageGross.Format()))
	}
	if v.ExemptionApplied {
		out = append(out, "An exemption has been applied because the register was set to apply it. This tool has not checked that the exemption is in force or that it is available to you.")
	}
	if n.Tax.RateOverride > 0 {
		out = append(out, fmt.Sprintf("This note uses a rate of %s entered on the note itself, not the register's figure.", n.Tax.RateOverride))
	}

	out = append(out,
		"The electronic way bill is a separate document. It is generated and filed on the government portal, not here; this tool only records the number it was given.",
		"This tool prepares and keeps documents. It is not advice, it files nothing, and it is connected to no portal.")
	return out
}

// ConstantRef is one figure this build ships with, where it came from, and what
// it is. The screen and the explainer both read this list rather than keeping
// their own copy, so the two cannot drift apart from the code.
type ConstantRef struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	What      string `json:"what"`
	Reference string `json:"reference"`
	Caution   string `json:"caution"`
	Applied   bool   `json:"applied"`
}

// Constants is every statutory figure this build carries, with its reference.
func Constants(rates Rates) []ConstantRef {
	rates = rates.Normalise()
	return []ConstantRef{
		{
			Name:      "RateRecipientReverseChargeDefault",
			Value:     rates.RecipientReverseCharge.String(),
			What:      "Rate applied where the recipient of the service accounts for the tax on the carriage under reverse charge.",
			Reference: "Notification No. 13/2017-Central Tax (Rate) dated 28 June 2017, as amended, read with Notification No. 11/2017-Central Tax (Rate) dated 28 June 2017, as amended.",
			Caution:   "Indicative and user-overridable. Confirm the current rate and the specified categories of recipient before relying on it.",
			Applied:   true,
		},
		{
			Name:      "RateGTAForwardNoITCDefault",
			Value:     rates.GTAForwardNoITC.String(),
			What:      "Rate applied where the agency has elected to pay the tax itself and does not take input tax credit.",
			Reference: "Notification No. 11/2017-Central Tax (Rate) dated 28 June 2017, as amended.",
			Caution:   "Indicative and user-overridable. The election has conditions attached; confirm them.",
			Applied:   true,
		},
		{
			Name:      "RateGTAForwardWithITCDefault",
			Value:     rates.GTAForwardWithITC.String(),
			What:      "Rate applied where the agency has elected to pay the tax itself and takes input tax credit.",
			Reference: "Notification No. 11/2017-Central Tax (Rate) dated 28 June 2017, as amended.",
			Caution:   "Indicative and user-overridable. The election has conditions attached; confirm them.",
			Applied:   true,
		},
		{
			Name:      "ExemptSingleCarriageGrossDefault",
			Value:     "₹" + rates.ExemptSingleCarriageGross.Format(),
			What:      "Gross amount charged for the transportation of goods in a single carriage below which an exemption was historically available.",
			Reference: "Notification No. 12/2017-Central Tax (Rate) dated 28 June 2017, entry against serial number 21 — since amended.",
			Caution:   "Indicative and NOT applied unless switched on. The entries this figure came from have been amended; check whether any such exemption is in force and available to you.",
			Applied:   rates.ApplyConsignmentExemption,
		},
		{
			Name:      "ExemptSingleConsigneeGrossDefault",
			Value:     "₹" + ExemptSingleConsigneeGrossDefault.Format(),
			What:      "The equivalent figure where the goods carried for a single consignee are what is in question.",
			Reference: "Notification No. 12/2017-Central Tax (Rate) dated 28 June 2017, entry against serial number 21 — since amended.",
			Caution:   "Carried for reference only. This build never applies it: a per-consignee test needs the whole carriage, and this register keeps one note at a time.",
			Applied:   false,
		},
		{
			Name:      "GSTINLength",
			Value:     "15 characters",
			What:      "Length of a registration number under the goods and services tax, whose fifteenth character this tool computes and compares.",
			Reference: "Rule 10 and the registration provisions of the Central Goods and Services Tax Rules, 2017.",
			Caution:   "A shape and check-character test only. This tool is connected to no register, so a number that passes may still not be the party's.",
			Applied:   true,
		},
		{
			Name:      "GramsPerQuintal",
			Value:     "100 kg",
			What:      "A quintal, the unit road freight in India is commonly quoted in.",
			Reference: "A unit of measure, not a statutory figure.",
			Caution:   "None. A tonne is 1,000 kg.",
			Applied:   true,
		},
	}
}
