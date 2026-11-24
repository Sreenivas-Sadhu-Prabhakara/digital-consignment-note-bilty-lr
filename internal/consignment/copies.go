package consignment

import (
	"fmt"
	"strings"
)

// The copy set.
//
// A consignment note is issued as a set, and the copies are not photocopies of
// each other. A paper bilty book is printed in triplicate on three colours
// precisely because the three parties need different things from the same
// transaction:
//
//   - The consignor's copy is the consignor's evidence that the goods were
//     handed over and accepted for carriage, and the consignor's record of what
//     the carriage cost. It carries the freight in full.
//
//   - The consignee's copy travels to the consignee and is what the goods are
//     claimed against at the destination. It carries the goods, the packages
//     and the vehicle, and what has to be paid before the goods are released —
//     which on a prepaid consignment is nothing, and the copy then shows no
//     freight at all. The rate the consignor negotiated is not the consignee's
//     business, and a prepaid consignee's copy showing it is how a carrier
//     loses a customer.
//
//   - The carrier's copy stays in the office. It carries everything: the
//     freight, the rate it was worked out from, the driver, and the block the
//     receiver signs at the destination.
//
// So which particulars go on which copy is a rule, not a layout choice, and it
// is derived here and tested.

// CopyKind is which copy of the set this is.
type CopyKind string

const (
	// CopyConsignor is the consignor's copy — the original.
	CopyConsignor CopyKind = "consignor"
	// CopyConsignee is the consignee's copy, which travels with or ahead of
	// the goods.
	CopyConsignee CopyKind = "consignee"
	// CopyCarrier is the carrier's own copy, retained in the office.
	CopyCarrier CopyKind = "carrier"
	// CopyProofOfDelivery is the proof of delivery raised from the delivery
	// record. Not one of the three, and issued after the carriage rather than
	// at booking.
	CopyProofOfDelivery CopyKind = "proof-of-delivery"
)

// CopyKinds is the set, in the order it is printed.
func CopyKinds() []CopyKind {
	return []CopyKind{CopyConsignor, CopyConsignee, CopyCarrier, CopyProofOfDelivery}
}

// Valid reports whether the copy is one this tool produces.
func (k CopyKind) Valid() bool {
	for _, known := range CopyKinds() {
		if k == known {
			return true
		}
	}
	return false
}

// Field is one labelled particular on a copy.
type Field struct {
	Label string `json:"label"`
	Value string `json:"value"`
	// Note is the smaller line under the value, where the figure needs one.
	Note string `json:"note"`
	// Strong marks a particular the copy sets apart.
	Strong bool `json:"strong"`
}

// MoneyLine is one line of the freight block.
type MoneyLine struct {
	Label  string `json:"label"`
	Amount Paise  `json:"amount"`
	Value  string `json:"value"`
	Note   string `json:"note"`
	Rule   bool   `json:"rule"`
	Strong bool   `json:"strong"`
}

// GoodsRow is one row of the goods table as a copy prints it.
type GoodsRow struct {
	No          int    `json:"no"`
	Packages    string `json:"packages"`
	Packing     string `json:"packing"`
	Description string `json:"description"`
	Weight      string `json:"weight"`
}

// SignatureBlock is one place somebody signs.
type SignatureBlock struct {
	For  string `json:"for"`
	Who  string `json:"who"`
	When string `json:"when"`
}

// Copy is one copy of the consignment note, laid out as a document.
type Copy struct {
	Kind  CopyKind `json:"kind"`
	Title string   `json:"title"`
	// For is who this copy belongs to, printed at the top of the page in the
	// place the colour of the paper used to occupy.
	For string `json:"for"`
	// Purpose is what this copy is for, in one sentence, printed small. A
	// consignee holding a copy that says what it is for is less likely to be
	// told something else at the counter.
	Purpose string `json:"purpose"`

	Number     string `json:"number"`
	Date       string `json:"date"`
	Status     string `json:"status"`
	Route      string `json:"route"`
	FreightTag string `json:"freightTag"`

	// TaxEndorsement is the line about who accounts for the tax on the
	// carriage, or that nobody has said.
	TaxEndorsement string `json:"taxEndorsement"`

	Parties   []PartyBlock `json:"parties"`
	Reference []Field      `json:"reference"`

	Goods      []GoodsRow `json:"goods"`
	GoodsTotal GoodsRow   `json:"goodsTotal"`
	SaidTo     string     `json:"saidToContain"`

	// Freight is the money block. Empty on a copy that does not carry money.
	Freight      []MoneyLine `json:"freight"`
	FreightWords string      `json:"freightWords"`
	// AmountToCollect is what the delivery clerk collects before releasing the
	// goods, printed on the copies that need it.
	AmountToCollect string `json:"amountToCollect"`

	Declared      string `json:"declaredValue"`
	DeclaredWords string `json:"declaredValueWords"`

	VehicleLine string `json:"vehicleLine"`
	DriverLine  string `json:"driverLine"`

	// POD is the delivery block. On the carrier's copy it is blank, to be
	// signed; on the proof of delivery it is filled in.
	POD        *ProofOfDelivery `json:"pod"`
	PODBlank   bool             `json:"podBlank"`
	Conditions []string         `json:"conditions"`
	Signatures []SignatureBlock `json:"signatures"`
	Notes      []string         `json:"notes"`
	Foot       string           `json:"foot"`
}

// PartyBlock is a party as a copy prints it.
type PartyBlock struct {
	Label string   `json:"label"`
	Lines []string `json:"lines"`
	Note  string   `json:"note"`
}

// CopySet is the whole set for one note.
type CopySet struct {
	Number string `json:"number"`
	Copies []Copy `json:"copies"`
	// Withheld names any copy the set does not include, and why. The proof of
	// delivery is withheld until the consignment has been delivered, because a
	// proof of delivery for an undelivered consignment is a forgery waiting to
	// happen.
	Withheld map[string]string `json:"withheld"`
}

// Build makes the copy set for a note.
func Build(n Note, rates Rates, carrier Carrier, width int) CopySet {
	n = n.Normalise()
	out := CopySet{Number: n.Number.Format(width), Withheld: map[string]string{}}

	for _, kind := range CopyKinds() {
		if kind == CopyProofOfDelivery && n.Status != StatusDelivered {
			out.Withheld[string(kind)] = fmt.Sprintf(
				"A proof of delivery is raised from the delivery record, and this consignment is %s. Record the delivery first.",
				strings.ToLower(n.Status.Name()))
			continue
		}
		out.Copies = append(out.Copies, BuildCopy(n, rates, carrier, kind, width))
	}
	return out
}

// Carrier is the transport office issuing the note — the one party that is the
// same on every note, so it is kept once in the register and not typed again.
type Carrier struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	PIN     string `json:"pin"`
	GSTIN   string `json:"gstin"`
	PAN     string `json:"pan"`
	Phone   string `json:"phone"`
	// Terms is the carrier's own conditions of carriage, printed on the back of
	// a paper bilty and at the foot of each copy here. Empty by default: this
	// tool will not invent somebody's contract terms for them.
	Terms string `json:"terms"`
}

// Normalise trims the carrier's details.
func (c Carrier) Normalise() Carrier {
	c.Name = strings.TrimSpace(c.Name)
	c.Address = strings.TrimSpace(c.Address)
	c.City = strings.TrimSpace(c.City)
	c.State = strings.TrimSpace(c.State)
	c.PIN = NormaliseCode(c.PIN)
	c.GSTIN = NormaliseCode(c.GSTIN)
	c.PAN = NormaliseCode(c.PAN)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Terms = strings.TrimSpace(c.Terms)
	return c
}

// Party is the carrier as a party, so it prints the same way the others do.
func (c Carrier) Party() Party {
	return Party{
		Name: c.Name, Address: c.Address, City: c.City, State: c.State,
		PIN: c.PIN, GSTIN: c.GSTIN, Phone: c.Phone,
	}
}

// BuildCopy makes one copy of the set.
func BuildCopy(n Note, rates Rates, carrier Carrier, kind CopyKind, width int) Copy {
	n = n.Normalise()
	carrier = carrier.Normalise()
	totals := Total(n)
	liability := Liable(n)
	tax := TaxOn(n, rates)

	if !kind.Valid() {
		kind = CopyConsignor
	}

	c := Copy{
		Kind:           kind,
		Number:         n.Number.Format(width),
		Date:           n.Date.Print(),
		Status:         n.Status.Name(),
		Route:          n.Route(),
		FreightTag:     n.Terms.Basis.Stamp(),
		TaxEndorsement: tax.Endorsement,
	}

	// What each copy is, and what it is for.
	switch kind {
	case CopyConsignor:
		c.Title = "Consignment note — consignor's copy"
		c.For = "Consignor's copy (original)"
		c.Purpose = "The consignor's evidence that these goods were handed over and accepted for carriage on the terms below, and the consignor's record of what the carriage cost."
	case CopyConsignee:
		c.Title = "Consignment note — consignee's copy"
		c.For = "Consignee's copy"
		c.Purpose = "Travels to the consignee. This is what the goods are claimed against at the destination, and what says whether anything is to be paid before they are released."
	case CopyCarrier:
		c.Title = "Consignment note — carrier's copy"
		c.For = "Carrier's copy (office)"
		c.Purpose = "Retained by the carrier. Carries the rate the freight was worked out from, the driver, and the block the receiver signs at the destination."
	case CopyProofOfDelivery:
		c.Title = "Proof of delivery"
		c.For = "Proof of delivery"
		c.Purpose = "Raised from the delivery record on the consignment note named below. It evidences what it says it evidences and no more."
	}

	// The parties. Every copy names the carrier, the consignor and the
	// consignee, because a copy that does not name all three is not evidence of
	// a contract of carriage between them.
	c.Parties = []PartyBlock{
		{Label: "Carrier", Lines: carrier.Party().Lines(), Note: carrierNote(carrier)},
		{Label: "Consignor", Lines: n.Consignor.Lines()},
		{Label: "Consignee", Lines: n.Consignee.Lines()},
	}
	if n.Terms.Basis == TermsBilledToThird {
		block := PartyBlock{Label: "Freight billed to", Lines: n.BillTo.Lines()}
		if len(block.Lines) == 0 && n.Terms.BilledToName != "" {
			block.Lines = []string{n.Terms.BilledToName}
		}
		c.Parties = append(c.Parties, block)
	}

	// The reference particulars, which every copy carries.
	c.Reference = []Field{
		{Label: "Consignment note no.", Value: n.Number.Format(width), Strong: true},
		{Label: "Date", Value: n.Date.Print(), Strong: true},
		{Label: "From", Value: n.From},
		{Label: "To", Value: n.To},
	}
	if n.PrivateMark != "" {
		c.Reference = append(c.Reference, Field{Label: "Private mark", Value: n.PrivateMark})
	}
	if n.ConsignorInvoiceNo != "" {
		c.Reference = append(c.Reference, Field{Label: "Consignor's invoice no.", Value: n.ConsignorInvoiceNo})
	}
	if n.EWayBillNumber != "" {
		c.Reference = append(c.Reference, Field{
			Label: "E-way bill no.", Value: n.EWayBillNumber,
			Note: "recorded here; the e-way bill itself is a separate document raised on the government portal",
		})
	}
	if expected := n.ExpectedDelivery(); !expected.IsZero() {
		c.Reference = append(c.Reference, Field{
			Label: "Expected at destination", Value: expected.Print(),
			Note: plural(n.TransitDays, "day in transit", "days in transit"),
		})
	}
	c.Reference = append(c.Reference, Field{Label: "Status", Value: n.Status.Name()})

	// The goods. Described as the consignor declared them, on every copy.
	for i, line := range n.Goods {
		c.Goods = append(c.Goods, GoodsRow{
			No:          i + 1,
			Packages:    plainInt(line.Packages),
			Packing:     line.Packing,
			Description: line.Description,
			Weight:      line.ActualGrams.String(),
		})
	}
	c.GoodsTotal = GoodsRow{
		Packages:    plainInt(totals.Packages),
		Description: "Total",
		Weight:      totals.ActualGrams.String(),
	}
	c.SaidTo = "Said to contain, as declared by the consignor. The carrier has not opened the packages and does not certify the contents, the quality or the condition of the goods inside them."

	// The vehicle, on every copy: the note is what says which vehicle carried
	// these goods.
	c.VehicleLine = vehicleLine(n.Vehicle)

	// The driver, only where the copy needs them. The carrier's copy and the
	// proof of delivery do; a consignor's and a consignee's copy are not the
	// place to print a driver's licence number.
	if kind == CopyCarrier || kind == CopyProofOfDelivery {
		c.DriverLine = driverLine(n.Driver)
	}

	// The money, and who sees it.
	c.Freight, c.FreightWords, c.AmountToCollect = freightBlock(kind, n, totals, liability, tax)

	// The declared value. The consignor declared it and the consignee needs it
	// to claim on, so both copies carry it; the carrier's copy carries it
	// because the carrier's liability may turn on it.
	if n.DeclaredValuePaise > 0 {
		c.Declared = "₹" + totals.DeclaredValuePaise.Format()
		c.DeclaredWords = totals.DeclaredInWords
	}

	// The delivery block.
	switch kind {
	case CopyProofOfDelivery:
		pod := NewProofOfDelivery(n, width)
		c.POD = &pod
	case CopyCarrier:
		c.PODBlank = true
		for _, cond := range Conditions() {
			c.Conditions = append(c.Conditions, cond.Name())
		}
	}

	c.Signatures = signatures(kind, n, carrier)
	c.Notes = copyNotes(kind, n, totals, liability, tax)
	c.Foot = foot(kind, carrier)
	return c
}

// freightBlock is the money on a copy — and whether there is any.
//
// This is the rule the copy set turns on. The consignee's copy of a prepaid
// consignment shows no freight: nothing is to be collected, and the rate the
// consignor negotiated is not the consignee's business. On a to-pay
// consignment the consignee's copy shows what has to be paid before the goods
// are released, because that is the one figure the consignee must know — but
// still not the rate it was worked out from.
func freightBlock(kind CopyKind, n Note, totals Totals, liability Liability, tax TaxView) (lines []MoneyLine, words, collect string) {
	full := kind == CopyConsignor || kind == CopyCarrier
	showRate := kind == CopyCarrier || (kind == CopyConsignor && totals.FreightFromRate)

	if kind == CopyProofOfDelivery {
		// A proof of delivery carries only what was collected and what is left.
		if liability.CollectAtDelivery > 0 || n.Delivery.FreightCollectedPaise > 0 {
			lines = append(lines,
				MoneyLine{Label: "To collect at delivery", Amount: liability.CollectAtDelivery, Value: liability.CollectAtDelivery.Format()},
				MoneyLine{Label: "Collected", Amount: n.Delivery.FreightCollectedPaise, Value: n.Delivery.FreightCollectedPaise.Format()},
			)
			remaining := liability.CollectAtDelivery - n.Delivery.FreightCollectedPaise
			if remaining < 0 {
				remaining = 0
			}
			lines = append(lines, MoneyLine{Label: "Still due", Amount: remaining, Value: remaining.Format(), Rule: true, Strong: true})
		}
		return lines, "", ""
	}

	if kind == CopyConsignee {
		switch n.Terms.Basis {
		case TermsToPay:
			collect = "₹" + liability.CollectAtDelivery.Format()
			lines = append(lines, MoneyLine{
				Label: "To pay before the goods are released", Amount: liability.CollectAtDelivery,
				Value: liability.CollectAtDelivery.Format(), Strong: true,
			})
			words = RupeesInWords(liability.CollectAtDelivery)
		case TermsPaid:
			collect = "Nothing — freight paid"
			lines = append(lines, MoneyLine{
				Label: "To pay before the goods are released", Amount: 0, Value: "Nil",
				Note: "the freight on this consignment was settled at booking", Strong: true,
			})
		case TermsBilledToThird:
			collect = "Nothing — freight billed to a third party"
			lines = append(lines, MoneyLine{
				Label: "To pay before the goods are released", Amount: 0, Value: "Nil",
				Note: "the freight on this consignment is billed to " + orRole(liability.PayableBy, "a third party named on this note"), Strong: true,
			})
		}
		return lines, words, collect
	}

	if !full {
		return nil, "", ""
	}

	// The consignor's and the carrier's copies carry the whole working.
	if showRate && totals.FreightFromRate && n.Charges.RatePaise > 0 {
		note := fmt.Sprintf("%s at ₹%s %s", totals.UnitsPrinted, n.Charges.RatePaise.Format(), totals.Basis.Unit())
		if totals.MinimumApplied {
			note += fmt.Sprintf("; charged on the minimum of %s, not the actual %s", totals.ChargeableGrams, totals.ActualGrams)
		}
		lines = append(lines, MoneyLine{Label: "Freight", Amount: totals.FreightPaise, Value: totals.FreightPaise.Format(), Note: note})
	} else {
		note := ""
		if !totals.FreightFromRate {
			note = "quoted for the consignment, not worked out from a rate"
		}
		lines = append(lines, MoneyLine{Label: "Freight", Amount: totals.FreightPaise, Value: totals.FreightPaise.Format(), Note: note})
	}

	for _, extra := range []struct {
		label  string
		amount Paise
	}{
		{"Loading", totals.LoadingPaise},
		{"Unloading", totals.UnloadingPaise},
		{"Door delivery", totals.DoorDeliveryPaise},
		{"Detention", totals.DetentionPaise},
		{firstNonEmpty(n.Charges.OtherLabel, "Other charges"), totals.OtherPaise},
	} {
		if extra.amount != 0 {
			lines = append(lines, MoneyLine{Label: extra.label, Amount: extra.amount, Value: extra.amount.Format()})
		}
	}

	lines = append(lines, MoneyLine{
		Label: "Gross freight", Amount: totals.GrossPaise, Value: totals.GrossPaise.Format(),
		Rule: true, Strong: true,
	})
	if totals.AdvancePaise != 0 {
		lines = append(lines,
			MoneyLine{Label: "Paid at booking", Amount: totals.AdvancePaise, Value: totals.AdvancePaise.Format()},
			MoneyLine{Label: "Balance", Amount: totals.BalancePaise, Value: totals.BalancePaise.Format(), Strong: true},
		)
	}

	// The tax, as a figure only where a position has been stated.
	if tax.Settled && tax.Amount > 0 {
		label := fmt.Sprintf("Tax on the carriage at %s", tax.Rate)
		note := tax.PayableBy
		if tax.Position.ReverseCharge() {
			note = "payable by the recipient under reverse charge — not collected with this freight"
		}
		lines = append(lines, MoneyLine{Label: label, Amount: tax.Amount, Value: tax.Amount.Format(), Note: note})
	} else if !tax.Settled {
		lines = append(lines, MoneyLine{
			Label: "Tax on the carriage", Amount: 0, Value: "not stated",
			Note: "no tax position has been stated on this note",
		})
	}

	words = totals.GrossInWords
	if totals.AdvancePaise != 0 {
		words = totals.BalanceInWords
	}

	collect = "₹" + liability.CollectAtDelivery.Format()
	if liability.CollectAtDelivery == 0 {
		collect = "Nothing to collect at delivery"
	}
	return lines, words, collect
}

func signatures(kind CopyKind, n Note, carrier Carrier) []SignatureBlock {
	switch kind {
	case CopyProofOfDelivery:
		return []SignatureBlock{
			{For: "Received by", Who: firstNonEmpty(n.Delivery.ReceiverName, "the receiver"), When: "Date: " + n.Delivery.Date.Print()},
			{For: "For the carrier", Who: firstNonEmpty(carrier.Name, "the carrier"), When: "Date:"},
		}
	case CopyCarrier:
		return []SignatureBlock{
			{For: "Goods booked by", Who: firstNonEmpty(carrier.Name, "the booking clerk"), When: "Date: " + n.Date.Print()},
			{For: "Driver", Who: firstNonEmpty(n.Driver.Name, "the driver"), When: "Date:"},
			{For: "Received by", Who: "name and signature of the person taking delivery", When: "Date:"},
		}
	case CopyConsignee:
		return []SignatureBlock{
			{For: "For the carrier", Who: firstNonEmpty(carrier.Name, "the carrier"), When: "Date: " + n.Date.Print()},
			{For: "Received by", Who: "name and signature of the person taking delivery", When: "Date:"},
		}
	default:
		return []SignatureBlock{
			{For: "Goods delivered to the carrier by", Who: firstNonEmpty(n.Consignor.Name, "the consignor"), When: "Date: " + n.Date.Print()},
			{For: "For the carrier", Who: firstNonEmpty(carrier.Name, "the carrier"), When: "Date: " + n.Date.Print()},
		}
	}
}

// copyNotes is what is printed small at the foot of a copy: the things that
// are true of this consignment and have to be said on paper.
func copyNotes(kind CopyKind, n Note, totals Totals, liability Liability, tax TaxView) []string {
	out := []string{liability.Explanation}

	if kind == CopyConsignee && n.Terms.Basis == TermsPaid {
		out = append(out, "The freight on this consignment does not appear on this copy because none of it is collected here. The consignor's copy carries it.")
	}
	if totals.MinimumApplied && (kind == CopyConsignor || kind == CopyCarrier) {
		out = append(out, fmt.Sprintf("Freight was charged on the minimum weight of %s booked for this vehicle, not on the %s the goods actually weigh.",
			totals.ChargeableGrams, totals.ActualGrams))
	}
	if n.DeclaredValuePaise == 0 {
		out = append(out, "No value was declared for these goods on this note.")
	} else if n.ValueDeclaredByName != "" {
		out = append(out, fmt.Sprintf("The value of ₹%s was declared by %s.", totals.DeclaredValuePaise.Format(), n.ValueDeclaredByName))
	}
	if !tax.Settled {
		out = append(out, "No tax position has been stated on this note. Whether the tax on this carriage is accounted for by the recipient under reverse charge or by the agency under forward charge has not been settled here.")
	}
	if n.EWayBillNumber == "" {
		out = append(out, "No e-way bill number is recorded on this note. The e-way bill is a separate document raised on the government portal and is not produced here.")
	}
	if n.Remarks != "" {
		out = append(out, "Remarks: "+n.Remarks)
	}
	if n.Status == StatusCancelled {
		out = append(out, fmt.Sprintf("CANCELLED on %s: %s. The number is retained and not reissued.", n.Cancellation.Date.Print(), n.Cancellation.Reason))
	}
	if n.Status == StatusVoid {
		out = append(out, fmt.Sprintf("VOID — this number was never used: %s. It is retained so that the series stays unbroken.", n.Cancellation.Reason))
	}
	return out
}

func foot(kind CopyKind, carrier Carrier) string {
	who := firstNonEmpty(carrier.Name, "the carrier")
	base := fmt.Sprintf("Issued by %s. Goods carried subject to the conditions of carriage.", who)
	if carrier.Terms != "" {
		base = fmt.Sprintf("Issued by %s. %s", who, carrier.Terms)
	}
	switch kind {
	case CopyConsignor:
		return base + " Consignor's copy — one of a set of three."
	case CopyConsignee:
		return base + " Consignee's copy — one of a set of three."
	case CopyCarrier:
		return base + " Carrier's copy — one of a set of three."
	default:
		return base
	}
}

func carrierNote(c Carrier) string {
	if c.Name == "" {
		return "the carrier's own details have not been entered in this register"
	}
	if c.GSTIN == "" && c.PAN == "" {
		return "no registration number recorded for the carrier"
	}
	if c.PAN != "" && c.GSTIN == "" {
		return "PAN " + c.PAN
	}
	return ""
}

func vehicleLine(v Vehicle) string {
	parts := nonEmpty(v.Printed(), v.Kind)
	if v.Owner != "" {
		parts = append(parts, "owner: "+v.Owner)
	}
	if len(parts) == 0 {
		return "no vehicle recorded"
	}
	return strings.Join(parts, " · ")
}

func driverLine(d Driver) string {
	parts := nonEmpty(d.Name)
	if d.LicenceNo != "" {
		parts = append(parts, "licence "+d.LicenceNo)
	}
	if d.Phone != "" {
		parts = append(parts, d.Phone)
	}
	if len(parts) == 0 {
		return "no driver recorded"
	}
	return strings.Join(parts, " · ")
}

func plainInt(n int) string {
	if n == 0 {
		return "0"
	}
	return fmt.Sprintf("%d", n)
}
