package consignment

import (
	"strconv"
	"strings"
)

// A consignment note — the bilty, or the lorry receipt — is the paper that
// travels with a load of goods on a road carriage. It says who handed the goods
// over, who is to receive them, what they are, how many packages they came in,
// what they weigh, what the carriage costs and who owes it, what the consignor
// says they are worth, and which vehicle and driver took them. It is the
// evidence of the contract of carriage between the consignor and the carrier,
// and it is issued in a set of copies so that each of the three has one.
//
// Every figure here is a whole number in its own smallest unit, and every date
// is a calendar day. Nothing is a floating-point rupee or a timestamp.

// Party is a consignor, a consignee or the party the freight is billed to.
type Party struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	PIN     string `json:"pin"`
	GSTIN   string `json:"gstin"`
	Phone   string `json:"phone"`
}

// Normalise trims a party and puts its registration number in standard form.
func (p Party) Normalise() Party {
	p.Name = strings.TrimSpace(p.Name)
	p.Address = strings.TrimSpace(p.Address)
	p.City = strings.TrimSpace(p.City)
	p.State = strings.TrimSpace(p.State)
	p.PIN = NormaliseCode(p.PIN)
	p.GSTIN = NormaliseCode(p.GSTIN)
	p.Phone = strings.TrimSpace(p.Phone)
	return p
}

// Named reports whether there is a party here at all.
func (p Party) Named() bool { return strings.TrimSpace(p.Name) != "" }

// Lines is the party as the block of lines a note prints it as.
func (p Party) Lines() []string {
	var out []string
	if p.Name != "" {
		out = append(out, p.Name)
	}
	for _, line := range strings.Split(p.Address, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}

	place := strings.TrimSpace(strings.Join(nonEmpty(p.City, p.State), ", "))
	if p.PIN != "" {
		place = strings.TrimSpace(place + " " + p.PIN)
	}
	if place != "" {
		out = append(out, place)
	}
	if p.GSTIN != "" {
		out = append(out, "GSTIN "+p.GSTIN)
	}
	if p.Phone != "" {
		out = append(out, "Phone "+p.Phone)
	}
	return out
}

func nonEmpty(in ...string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Goods is one line of the goods description: what the packages hold, how many
// of them there are, how they are packed and what they weigh. A carrier
// describes goods as the consignor said they are, because the carrier did not
// open the packages — which is why the note says "said to contain".
type Goods struct {
	Description string `json:"description"`
	Packages    int    `json:"packages"`
	Packing     string `json:"packing"`
	ActualGrams Grams  `json:"actualGrams"`
}

// Normalise trims a goods line and stops a negative count or weight.
func (g Goods) Normalise() Goods {
	g.Description = strings.TrimSpace(g.Description)
	g.Packing = strings.TrimSpace(g.Packing)
	if g.Packages < 0 {
		g.Packages = 0
	}
	if g.ActualGrams < 0 {
		g.ActualGrams = 0
	}
	return g
}

// Empty reports whether the line says nothing at all, in which case it is
// dropped rather than printed as a blank row on the note.
func (g Goods) Empty() bool {
	return g.Description == "" && g.Packages == 0 && g.ActualGrams == 0 && g.Packing == ""
}

// PackingKinds are the ways road freight is packed, offered as a list so the
// register can be searched on them.
func PackingKinds() []string {
	return []string{"Bags", "Bales", "Bundles", "Cartons", "Cases", "Crates", "Drums", "Rolls", "Loose", "Pallets", "Tins"}
}

// Charges is how the freight on this consignment was worked out and what else
// is being charged with it.
type Charges struct {
	Basis Basis `json:"basis"`
	// RatePaise is the rate on the chosen basis — so much per quintal, per
	// tonne, per kilogram, per package, or for the trip.
	RatePaise Paise `json:"ratePaise"`
	// MinimumChargedGrams is the weight the carrier books the trip at whatever
	// the load comes to. Freight is charged on the greater of this and the
	// actual weight, and the note shows both.
	MinimumChargedGrams Grams `json:"minimumChargedGrams"`
	// FreightOverridden is set where the carrier quoted a figure outright
	// rather than a rate. The figure then stands and the rate is not applied,
	// but both are kept so the note can show which was used.
	FreightOverridden    bool  `json:"freightOverridden"`
	FreightOverridePaise Paise `json:"freightOverridePaise"`

	LoadingPaise      Paise  `json:"loadingPaise"`
	UnloadingPaise    Paise  `json:"unloadingPaise"`
	DoorDeliveryPaise Paise  `json:"doorDeliveryPaise"`
	DetentionPaise    Paise  `json:"detentionPaise"`
	OtherPaise        Paise  `json:"otherPaise"`
	OtherLabel        string `json:"otherLabel"`

	// AdvancePaise is what was collected at booking. On a to-pay consignment it
	// is usually nothing and occasionally a part payment; the balance is what
	// is collected at the destination.
	AdvancePaise Paise `json:"advancePaise"`
}

// Normalise fills in a basis and stops negative charges.
func (c Charges) Normalise() Charges {
	if !c.Basis.Valid() {
		c.Basis = BasisPerQuintal
	}
	c.OtherLabel = strings.TrimSpace(c.OtherLabel)

	for _, f := range []*Paise{
		&c.RatePaise, &c.FreightOverridePaise, &c.LoadingPaise, &c.UnloadingPaise,
		&c.DoorDeliveryPaise, &c.DetentionPaise, &c.OtherPaise, &c.AdvancePaise,
	} {
		if *f < 0 {
			*f = 0
		}
	}
	if c.MinimumChargedGrams < 0 {
		c.MinimumChargedGrams = 0
	}
	return c
}

// Vehicle is the goods carriage the consignment went on. The registration
// number is a particular of the note: the note is what says which vehicle
// carried these goods.
type Vehicle struct {
	Number string `json:"number"`
	Kind   string `json:"kind"`
	Owner  string `json:"owner"`
}

// Normalise puts the registration number in standard form.
func (v Vehicle) Normalise() Vehicle {
	v.Number = NormaliseCode(v.Number)
	v.Kind = strings.TrimSpace(v.Kind)
	v.Owner = strings.TrimSpace(v.Owner)
	return v
}

// Printed is the registration number spaced the way it is painted on the
// vehicle.
func (v Vehicle) Printed() string { return PrintVehicleNumber(v.Number) }

// VehicleKinds are the goods carriages a small transport office books on,
// offered as a list rather than typed each time.
func VehicleKinds() []string {
	return []string{
		"Tata Ace / pickup", "407 / LCV", "709 / 6-tyre", "1109 / 10-tyre",
		"19 ft container", "22 ft container", "32 ft single axle", "32 ft multi axle",
		"Trailer", "Tanker", "Tipper",
	}
}

// Driver is who took the vehicle out.
type Driver struct {
	Name      string `json:"name"`
	LicenceNo string `json:"licenceNo"`
	Phone     string `json:"phone"`
}

// Normalise trims the driver's details.
func (d Driver) Normalise() Driver {
	d.Name = strings.TrimSpace(d.Name)
	d.LicenceNo = strings.ToUpper(strings.TrimSpace(d.LicenceNo))
	d.Phone = strings.TrimSpace(d.Phone)
	return d
}

// Note is one consignment note.
type Note struct {
	Number Number `json:"number"`
	Date   Date   `json:"date"`
	Status Status `json:"status"`

	// From and To are the booking station and the destination station — the
	// place of origin and the place of delivery, which are particulars of the
	// note and not the same as the parties' own addresses.
	From string `json:"from"`
	To   string `json:"to"`

	Consignor Party `json:"consignor"`
	Consignee Party `json:"consignee"`
	// BillTo is the party the freight is billed to where that is neither the
	// consignor nor the consignee.
	BillTo Party `json:"billTo"`

	Goods               []Goods `json:"goods"`
	PrivateMark         string  `json:"privateMark"`
	ConsignorInvoiceNo  string  `json:"consignorInvoiceNo"`
	DeclaredValuePaise  Paise   `json:"declaredValuePaise"`
	ValueDeclaredByName string  `json:"valueDeclaredByName"`

	Terms   Terms   `json:"terms"`
	Charges Charges `json:"charges"`

	Vehicle Vehicle `json:"vehicle"`
	Driver  Driver  `json:"driver"`

	Tax TaxStated `json:"tax"`

	// EWayBillNumber is recorded, not generated. The electronic way bill is a
	// separate document raised on the government portal; this field is where
	// the number it was given is kept so the note and the way bill can be
	// found together.
	EWayBillNumber string `json:"eWayBillNumber"`

	// TransitDays is how many days the carriage is expected to take, from which
	// the expected delivery date is worked out.
	TransitDays int `json:"transitDays"`

	Delivery     Delivery     `json:"delivery"`
	Cancellation Cancellation `json:"cancellation"`

	Remarks string `json:"remarks"`
}

// Normalise puts a note in standard form: trimmed, with empty goods lines
// dropped, a status that exists, and no negative figures.
func (n Note) Normalise() Note {
	n.Number.Series = NormaliseSeries(n.Number.Series)
	if n.Number.Seq < 0 {
		n.Number.Seq = 0
	}
	if !n.Status.Valid() {
		n.Status = StatusBooked
	}

	n.From = strings.TrimSpace(n.From)
	n.To = strings.TrimSpace(n.To)
	n.PrivateMark = strings.TrimSpace(n.PrivateMark)
	n.ConsignorInvoiceNo = strings.TrimSpace(n.ConsignorInvoiceNo)
	n.ValueDeclaredByName = strings.TrimSpace(n.ValueDeclaredByName)
	n.EWayBillNumber = strings.ToUpper(strings.TrimSpace(n.EWayBillNumber))
	n.Remarks = strings.TrimSpace(n.Remarks)

	n.Consignor = n.Consignor.Normalise()
	n.Consignee = n.Consignee.Normalise()
	n.BillTo = n.BillTo.Normalise()

	n.Terms = n.Terms.Normalise()
	n.Charges = n.Charges.Normalise()
	n.Vehicle = n.Vehicle.Normalise()
	n.Driver = n.Driver.Normalise()
	n.Tax = n.Tax.Normalise()
	n.Delivery = n.Delivery.Normalise()
	n.Cancellation = n.Cancellation.Normalise()

	if n.DeclaredValuePaise < 0 {
		n.DeclaredValuePaise = 0
	}
	if n.TransitDays < 0 {
		n.TransitDays = 0
	}

	kept := make([]Goods, 0, len(n.Goods))
	for _, line := range n.Goods {
		line = line.Normalise()
		if !line.Empty() {
			kept = append(kept, line)
		}
	}
	n.Goods = kept
	return n
}

// Totals is every figure derived from the note, worked out in one place so the
// screen, the copies and the register cannot each arrive at their own.
type Totals struct {
	Packages        int    `json:"packages"`
	GoodsLines      int    `json:"goodsLines"`
	ActualGrams     Grams  `json:"actualGrams"`
	ChargeableGrams Grams  `json:"chargeableGrams"`
	MinimumApplied  bool   `json:"minimumApplied"`
	Basis           Basis  `json:"basis"`
	UnitsPrinted    string `json:"unitsPrinted"`

	FreightPaise Paise `json:"freightPaise"`
	// FreightFromRate is false where the carrier's quoted figure was used
	// instead of the rate, which the note states so that nobody reads the rate
	// as having produced the figure.
	FreightFromRate bool `json:"freightFromRate"`

	LoadingPaise      Paise `json:"loadingPaise"`
	UnloadingPaise    Paise `json:"unloadingPaise"`
	DoorDeliveryPaise Paise `json:"doorDeliveryPaise"`
	DetentionPaise    Paise `json:"detentionPaise"`
	OtherPaise        Paise `json:"otherPaise"`

	// GrossPaise is the gross amount charged for the transportation of these
	// goods. It is the figure the tax position is worked out on and the figure
	// the parties argue about, so it is derived once here.
	GrossPaise   Paise `json:"grossPaise"`
	AdvancePaise Paise `json:"advancePaise"`
	BalancePaise Paise `json:"balancePaise"`

	DeclaredValuePaise Paise `json:"declaredValuePaise"`

	GrossInWords    string `json:"grossInWords"`
	BalanceInWords  string `json:"balanceInWords"`
	DeclaredInWords string `json:"declaredInWords"`
	WeightInWords   string `json:"weightInWords"`
}

// Total works every derived figure out from the note.
func Total(n Note) Totals {
	n = n.Normalise()

	out := Totals{Basis: n.Charges.Basis, GoodsLines: len(n.Goods)}
	for _, line := range n.Goods {
		out.Packages += line.Packages
		out.ActualGrams += line.ActualGrams
	}

	out.ChargeableGrams = ChargeableWeight(out.ActualGrams, n.Charges.MinimumChargedGrams)
	out.MinimumApplied = n.Charges.MinimumChargedGrams > out.ActualGrams
	out.UnitsPrinted = UnitsPrinted(n.Charges.Basis, out.ChargeableGrams, out.Packages)

	if n.Charges.FreightOverridden {
		out.FreightPaise = n.Charges.FreightOverridePaise
		out.FreightFromRate = false
	} else {
		out.FreightPaise = Freight(n.Charges.RatePaise, n.Charges.Basis, out.ChargeableGrams, out.Packages)
		out.FreightFromRate = true
	}

	out.LoadingPaise = n.Charges.LoadingPaise
	out.UnloadingPaise = n.Charges.UnloadingPaise
	out.DoorDeliveryPaise = n.Charges.DoorDeliveryPaise
	out.DetentionPaise = n.Charges.DetentionPaise
	out.OtherPaise = n.Charges.OtherPaise

	out.GrossPaise = out.FreightPaise + out.LoadingPaise + out.UnloadingPaise +
		out.DoorDeliveryPaise + out.DetentionPaise + out.OtherPaise

	out.AdvancePaise = n.Charges.AdvancePaise
	out.BalancePaise = out.GrossPaise - out.AdvancePaise
	out.DeclaredValuePaise = n.DeclaredValuePaise

	out.GrossInWords = RupeesInWords(out.GrossPaise)
	out.BalanceInWords = RupeesInWords(out.BalancePaise)
	out.DeclaredInWords = RupeesInWords(out.DeclaredValuePaise)
	out.WeightInWords = KilogramsInWords(out.ActualGrams)
	return out
}

// ExpectedDelivery is the day the consignment is expected at the destination:
// the booking date plus the transit days, counted on the calendar so a note
// booked on 27 February with three days' transit lands on 1 March, or on 2
// March in a leap year.
func (n Note) ExpectedDelivery() Date {
	if n.Date.IsZero() || n.TransitDays <= 0 {
		return Date{}
	}
	return n.Date.AddDays(n.TransitDays)
}

// Route is the carriage in one line, for the register: "Bengaluru to Hubballi".
func (n Note) Route() string {
	from, to := strings.TrimSpace(n.From), strings.TrimSpace(n.To)
	switch {
	case from != "" && to != "":
		return from + " to " + to
	case from != "":
		return "from " + from
	case to != "":
		return "to " + to
	default:
		return ""
	}
}

// GoodsSummary is the goods in one line, for the register.
func (n Note) GoodsSummary() string {
	var parts []string
	for _, line := range n.Goods {
		if line.Description != "" {
			parts = append(parts, line.Description)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 2 {
		return strings.Join(parts[:2], ", ") + " and " + plural(len(parts)-2, "other line", "other lines")
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
