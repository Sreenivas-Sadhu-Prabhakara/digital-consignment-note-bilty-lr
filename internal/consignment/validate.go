package consignment

import (
	"fmt"
	"strings"
)

// What is wrong with a consignment note, sorted into what must be fixed before
// it is issued and what is worth a second look.
//
// The line between the two is drawn at one place: a blocker is something that
// would make the note *wrong* or make it fail as the document it is meant to
// be, and a check is something that would merely make it incomplete. A note
// with no consignee named is not a consignment note. A note with no driver's
// phone number is a consignment note with no driver's phone number.

// Severity is how badly something is wrong.
type Severity string

const (
	// SeverityBlocker stops the note being issued.
	SeverityBlocker Severity = "blocker"
	// SeverityCheck is worth looking at before it goes out.
	SeverityCheck Severity = "check"
	// SeverityNote is something the note will say on its face, so that a reader
	// is not left to assume.
	SeverityNote Severity = "note"
)

// Finding is one thing that is wrong, or worth a look.
type Finding struct {
	Severity Severity `json:"severity"`
	// Field is which particular it is about, so the screen can mark it.
	Field string `json:"field"`
	What  string `json:"what"`
	Fix   string `json:"fix"`
}

// Report is everything found on one note.
type Report struct {
	Findings []Finding `json:"findings"`
	Blockers int       `json:"blockers"`
	Checks   int       `json:"checks"`
	Notes    int       `json:"notes"`
	// Issuable reports whether the note can be issued: no blockers.
	Issuable bool `json:"issuable"`
	// Verdict is the one line the screen shows.
	Verdict string `json:"verdict"`
}

// Check works out everything wrong with a note.
func Check(n Note, rates Rates, carrier Carrier) Report {
	n = n.Normalise()
	carrier = carrier.Normalise()
	totals := Total(n)

	var out []Finding
	add := func(sev Severity, field, what, fix string) {
		out = append(out, Finding{Severity: sev, Field: field, What: what, Fix: fix})
	}

	// A void number is not a consignment and is not checked as one.
	if n.Status == StatusVoid {
		if strings.TrimSpace(n.Cancellation.Reason) == "" {
			add(SeverityBlocker, "cancellation.reason",
				"This number is marked void with no reason given.",
				"Say why it was never used — a spoiled leaf, or a number skipped and accounted for afterwards.")
		}
		return report(out)
	}

	// The number itself.
	if n.Number.Seq <= 0 {
		add(SeverityBlocker, "number",
			"This note has no number.",
			"Allocate the next number in the series. A consignment note without a number is outside the register and cannot be audited.")
	}
	if err := CheckSeries(n.Number.Series); err != nil {
		add(SeverityBlocker, "number.series", err.Error(), "Use letters, digits, a hyphen or a slash.")
	}
	if n.Date.IsZero() {
		add(SeverityBlocker, "date",
			"This note has no date.",
			"Date it the day the goods were handed over.")
	}

	// The parties. All three of the carrier, the consignor and the consignee
	// have to be identified: the note is the evidence of a contract between
	// them, and one without a party to it is not evidence of anything.
	if carrier.Name == "" {
		add(SeverityBlocker, "carrier.name",
			"The carrier's own name is not in this register, so no copy can say who issued the note.",
			"Enter the transport office's name and address once, in the register's own details.")
	}
	if !n.Consignor.Named() {
		add(SeverityBlocker, "consignor.name",
			"No consignor is named.",
			"Name whoever handed the goods over.")
	}
	if !n.Consignee.Named() {
		add(SeverityBlocker, "consignee.name",
			"No consignee is named.",
			"Name whoever is to receive the goods. A consignment with nobody to deliver it to cannot be carried.")
	}

	// The goods.
	if len(n.Goods) == 0 {
		add(SeverityBlocker, "goods",
			"No goods are described.",
			"Add at least one line saying what is being carried, in how many packages, at what weight.")
	}
	if len(n.Goods) > 0 && totals.Packages == 0 {
		add(SeverityBlocker, "goods.packages",
			"The goods are described but no packages are counted.",
			"Say how many packages there are. The count is what is checked at delivery.")
	}
	if len(n.Goods) > 0 && totals.ActualGrams == 0 {
		add(SeverityCheck, "goods.weight",
			"The goods are described but nothing is weighed.",
			"Enter the weight. On every basis but per-package and lump sum, it is what the freight is charged on.")
	}
	for i, line := range n.Goods {
		if line.Description == "" {
			add(SeverityBlocker, fmt.Sprintf("goods.%d.description", i),
				fmt.Sprintf("Line %d has packages or weight but no description of the goods.", i+1),
				"Describe what the packages hold, as the consignor declared it.")
		}
		if line.Packages == 0 && line.ActualGrams > 0 {
			add(SeverityCheck, fmt.Sprintf("goods.%d.packages", i),
				fmt.Sprintf("Line %d is weighed but no packages are counted on it.", i+1),
				"Count the packages, or say the goods are loose.")
		}
	}

	// The route. The place of origin and the place of delivery are particulars
	// of the note and are not the parties' addresses.
	if n.From == "" {
		add(SeverityBlocker, "from",
			"The place the goods were booked from is not stated.",
			"Name the booking station. It is a particular of the note in its own right, not the consignor's address.")
	}
	if n.To == "" {
		add(SeverityBlocker, "to",
			"The place of delivery is not stated.",
			"Name the destination station.")
	}

	// A cancelled note is not going anywhere, so the checks that look forward
	// to a carriage — no vehicle yet, no freight yet, an advance short of the
	// gross, no transit time — are not asked of it. The number it keeps is the
	// whole point of it, and that has already been checked.
	live := n.Status != StatusCancelled

	// The vehicle.
	switch {
	case n.Vehicle.Number == "" && n.Status.Carried():
		add(SeverityBlocker, "vehicle.number",
			"This consignment is marked as carried but no vehicle is recorded.",
			"Enter the registration number of the goods carriage. The note is what says which vehicle took these goods.")
	case n.Vehicle.Number == "" && live:
		add(SeverityCheck, "vehicle.number",
			"No vehicle is recorded yet.",
			"Enter the registration number before the goods are loaded out.")
	case n.Vehicle.Number == "":
		// A cancelled note with no vehicle on it is in order.
	default:
		if err := CheckVehicleNumber(n.Vehicle.Number); err != nil {
			add(SeverityCheck, "vehicle.number", err.Error(),
				"Check the number against the vehicle. This tool is connected to no vehicle register, so it only checks the shape.")
		}
	}
	if n.Driver.Name == "" && n.Status.Carried() {
		add(SeverityCheck, "driver.name",
			"This consignment is marked as carried but no driver is recorded.",
			"Record who took the vehicle out. It is the first thing asked for when a consignment goes missing.")
	}

	// The freight terms.
	switch n.Terms.Basis {
	case TermsToPay:
		if !n.Consignee.Named() {
			add(SeverityBlocker, "terms.basis",
				"The freight is to pay, which means the consignee settles it before the goods are released — but no consignee is named.",
				"Name the consignee, or change the term to paid or billed to a third party.")
		}
		if totals.GrossPaise == 0 {
			add(SeverityBlocker, "charges",
				"The freight is to pay but comes to nothing, so there is nothing to collect at the destination.",
				"Enter the rate or the quoted freight, or change the term to paid.")
		}
	case TermsBilledToThird:
		if !n.BillTo.Named() && n.Terms.BilledToName == "" {
			add(SeverityBlocker, "billTo.name",
				"The freight is billed to a third party but no third party is named.",
				"Name the party the freight is billed to. Nothing is collected at either end, so the note is the only record of who owes it.")
		}
	case TermsPaid:
		if live && totals.GrossPaise > 0 && totals.AdvancePaise < totals.GrossPaise {
			add(SeverityCheck, "charges.advancePaise",
				fmt.Sprintf("The note is stamped paid but only ₹%s of the ₹%s gross freight was taken at booking.",
					totals.AdvancePaise.Format(), totals.GrossPaise.Format()),
				"Enter the full amount as paid at booking, or change the term to to-pay for the balance.")
		}
	}

	// The charges.
	if live && totals.GrossPaise == 0 {
		add(SeverityCheck, "charges",
			"No freight is charged on this consignment.",
			"Enter a rate on the chosen basis, or the figure the carriage was quoted at.")
	}
	if !n.Charges.FreightOverridden && n.Charges.RatePaise == 0 && totals.GrossPaise > 0 {
		add(SeverityCheck, "charges.ratePaise",
			"There is no freight rate, so the gross is made up of the other charges alone.",
			"Enter the rate, or record the carriage as quoted for the consignment.")
	}
	if n.Charges.Basis == BasisPerPackage && totals.Packages == 0 && n.Charges.RatePaise > 0 {
		add(SeverityBlocker, "charges.basis",
			"The freight is charged per package but no packages are counted, so the rate has nothing to multiply.",
			"Count the packages, or change the basis.")
	}
	if totals.BalancePaise < 0 {
		add(SeverityCheck, "charges.advancePaise",
			fmt.Sprintf("More was taken at booking (₹%s) than the gross freight comes to (₹%s).",
				totals.AdvancePaise.Format(), totals.GrossPaise.Format()),
			"Check the advance against the charges.")
	}

	// The declared value.
	if n.DeclaredValuePaise == 0 {
		add(SeverityNote, "declaredValuePaise",
			"No value is declared for these goods.",
			"A note with no declared value says so on its face. What the carrier's liability is in that case is a matter for the conditions of carriage and the law, not for this tool.")
	} else if n.ValueDeclaredByName == "" {
		add(SeverityCheck, "valueDeclaredByName",
			"A value is declared but nobody is named as having declared it.",
			"Name whoever declared it. A declared value is the consignor's statement, not the carrier's.")
	}

	// The registration numbers. The recipient's matters most: whether the
	// recipient is registered is the fact the reverse-charge question turns on.
	for field, party := range map[string]Party{
		"carrier.gstin":   carrier.Party(),
		"consignor.gstin": n.Consignor,
		"consignee.gstin": n.Consignee,
		"billTo.gstin":    n.BillTo,
	} {
		if err := CheckGSTIN(party.GSTIN); err != nil {
			add(SeverityCheck, field, err.Error(), "Check the number against the party's own registration certificate.")
		}
	}
	if err := CheckPAN(carrier.PAN); err != nil {
		add(SeverityCheck, "carrier.pan", err.Error(), "Check the account number.")
	}
	for field, pin := range map[string]string{
		"consignor.pin": n.Consignor.PIN,
		"consignee.pin": n.Consignee.PIN,
	} {
		if err := CheckPIN(pin); err != nil {
			add(SeverityCheck, field, err.Error(), "Check the postal index number.")
		}
	}

	// The tax position.
	tax := TaxOn(n, rates)
	if !tax.Settled {
		add(SeverityNote, "tax.position",
			"No tax position has been stated on this note, so the note will print that none was stated.",
			"Choose a position once you know the agency's own election and the status of the recipient of the service. This tool will not choose for you.")
	} else {
		if tax.Position == PositionNoTaxCharged && n.Tax.Reason == "" {
			add(SeverityBlocker, "tax.reason",
				"The note says no tax is charged but gives no reason.",
				"State the reason. It is the reason, and not the note, that will have to stand up.")
		}
		if tax.Position.ReverseCharge() && tax.RecipientGSTIN == "" {
			add(SeverityCheck, "tax.position",
				fmt.Sprintf("The note says the tax is accounted for by the recipient under reverse charge, but no registration number is recorded for %s.", tax.RecipientDescription),
				"Record the recipient's registration number. Whether the recipient falls in a specified category is the fact this position turns on.")
		}
		if tax.ExemptionApplied {
			add(SeverityCheck, "tax.position",
				"An exemption has been applied to this note because the register is set to apply it.",
				"This tool has not checked that the exemption is in force or available to you. Confirm it.")
		}
	}

	// The e-way bill, recorded and not generated.
	if n.EWayBillNumber == "" {
		add(SeverityNote, "eWayBillNumber",
			"No e-way bill number is recorded against this note.",
			"The e-way bill is a separate document, generated and filed on the government portal. If one was raised for this consignment, enter its number here so the two can be found together.")
	}

	// The delivery.
	if n.Status == StatusDelivered {
		if err := CheckDelivery(n, n.Delivery); err != nil {
			add(SeverityBlocker, "delivery", err.Error(), "Complete the delivery record. It is what the proof of delivery is raised from.")
		}
		if short := Shortage(n, n.Delivery); short > 0 {
			add(SeverityCheck, "delivery.packagesReceived",
				fmt.Sprintf("%s did not arrive: %d of %d packages were received.",
					plural(short, "package", "packages"), n.Delivery.PackagesReceived, totals.Packages),
				"Make sure the remark on the record says what happened. A short delivery with no remark is where a claim is lost.")
		}
		if !n.Delivery.PackagesCounted {
			add(SeverityCheck, "delivery.packagesCounted",
				"The packages were not counted at delivery, so the proof of delivery is not evidence that all of them arrived.",
				"Count them at the door. It takes a minute and it is the whole value of the document.")
		}
		if n.Terms.Basis == TermsToPay {
			liability := Liable(n)
			if n.Delivery.FreightCollectedPaise < liability.CollectAtDelivery {
				add(SeverityCheck, "delivery.freightCollectedPaise",
					fmt.Sprintf("The goods were released on a to-pay note with ₹%s collected against ₹%s due.",
						n.Delivery.FreightCollectedPaise.Format(), liability.CollectAtDelivery.Format()),
					"Record what was actually collected. On a to-pay note the goods are the carrier's only hold on the freight.")
			}
		}
	} else if n.Delivery.Recorded() {
		add(SeverityCheck, "delivery",
			fmt.Sprintf("There is a delivery record on this note but it is still marked %s.", strings.ToLower(n.Status.Name())),
			"Mark it delivered, or clear the record.")
	}

	if n.Status == StatusCancelled && strings.TrimSpace(n.Cancellation.Reason) == "" {
		add(SeverityBlocker, "cancellation.reason",
			"This note is cancelled with no reason given.",
			"Say why. A cancelled note keeps its number, and the register has to say what became of it.")
	}

	// The expected delivery date.
	if n.TransitDays == 0 && n.Status.Open() {
		add(SeverityNote, "transitDays",
			"No transit time is stated, so the note shows no expected delivery date.",
			"Enter the days the carriage is expected to take.")
	}
	return report(out)
}

// report counts the findings and writes the verdict.
func report(findings []Finding) Report {
	out := Report{Findings: sortFindings(findings)}
	for _, f := range out.Findings {
		switch f.Severity {
		case SeverityBlocker:
			out.Blockers++
		case SeverityCheck:
			out.Checks++
		default:
			out.Notes++
		}
	}
	out.Issuable = out.Blockers == 0

	switch {
	case out.Blockers > 0 && out.Checks > 0:
		out.Verdict = fmt.Sprintf("Not ready to issue: %s and %s.",
			plural(out.Blockers, "thing has to be fixed", "things have to be fixed"),
			plural(out.Checks, "thing is worth a look", "things are worth a look"))
	case out.Blockers > 0:
		out.Verdict = fmt.Sprintf("Not ready to issue: %s.",
			plural(out.Blockers, "thing has to be fixed", "things have to be fixed"))
	case out.Checks > 0:
		out.Verdict = fmt.Sprintf("Ready to issue, with %s worth a look.",
			plural(out.Checks, "thing", "things"))
	default:
		out.Verdict = "Ready to issue."
	}
	return out
}

// sortFindings puts the blockers first, then the checks, then the notes,
// keeping the order within each group so the screen does not jump about as a
// note is typed in.
func sortFindings(in []Finding) []Finding {
	out := make([]Finding, 0, len(in))
	for _, want := range []Severity{SeverityBlocker, SeverityCheck, SeverityNote} {
		for _, f := range in {
			if f.Severity == want {
				out = append(out, f)
			}
		}
	}
	return out
}
