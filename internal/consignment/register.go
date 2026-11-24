package consignment

import (
	"fmt"
	"sort"
	"strings"
)

// The register: every consignment note the office has issued, searchable, with
// the money that is still outstanding against it and the serial audit over the
// top. A handwritten bilty book cannot be searched at all; that is most of what
// this is for.

// Row is one note as the register lists it.
type Row struct {
	Number    string `json:"number"`
	Series    string `json:"series"`
	Seq       int    `json:"seq"`
	Date      Date   `json:"date"`
	DatePrint string `json:"datePrint"`

	Status     Status `json:"status"`
	StatusName string `json:"statusName"`
	Stage      int    `json:"stage"`
	StageOf    int    `json:"stageOf"`

	Consignor string `json:"consignor"`
	Consignee string `json:"consignee"`
	Route     string `json:"route"`
	Goods     string `json:"goods"`

	Packages int    `json:"packages"`
	Weight   string `json:"weight"`

	Terms      TermsBasis `json:"terms"`
	TermsStamp string     `json:"termsStamp"`

	GrossPaise       Paise `json:"grossPaise"`
	OutstandingPaise Paise `json:"outstandingPaise"`
	DeclaredPaise    Paise `json:"declaredPaise"`

	Vehicle     string `json:"vehicle"`
	Driver      string `json:"driver"`
	EWayBill    string `json:"eWayBill"`
	TaxPosition string `json:"taxPosition"`
	TaxSettled  bool   `json:"taxSettled"`
	TaxPaise    Paise  `json:"taxPaise"`

	ExpectedOn  Date `json:"expectedOn"`
	DeliveredOn Date `json:"deliveredOn"`
	DaysLate    int  `json:"daysLate"`
	Overdue     bool `json:"overdue"`

	Blockers int  `json:"blockers"`
	Checks   int  `json:"checks"`
	Issuable bool `json:"issuable"`
}

// Summary is the register at a glance.
type Summary struct {
	Notes int `json:"notes"`

	Booked    int `json:"booked"`
	InTransit int `json:"inTransit"`
	Arrived   int `json:"arrived"`
	Delivered int `json:"delivered"`
	Cancelled int `json:"cancelled"`
	Void      int `json:"void"`

	Open    int `json:"open"`
	Overdue int `json:"overdue"`

	GrossPaise       Paise `json:"grossPaise"`
	OutstandingPaise Paise `json:"outstandingPaise"`
	// ToCollectPaise is the money a delivery clerk is due to collect at the
	// door on consignments not yet delivered. It is the figure a small
	// transport office most often loses track of.
	ToCollectPaise Paise `json:"toCollectPaise"`
	DeclaredPaise  Paise `json:"declaredPaise"`

	// TaxUnstated is how many notes have no tax position on them.
	TaxUnstated int `json:"taxUnstated"`
	// WithBlockers is how many notes are not ready to issue.
	WithBlockers int `json:"withBlockers"`
	// WithoutEWayBill is how many notes carry no e-way bill number. Recorded,
	// not required: whether one was needed is not for this tool to say.
	WithoutEWayBill int `json:"withoutEWayBill"`
}

// Filter is how the register is narrowed.
type Filter struct {
	// Text matches the number, either party, the route, the goods, the vehicle
	// or the e-way bill number, case-insensitively.
	Text   string   `json:"text"`
	Series string   `json:"series"`
	Status []Status `json:"status"`
	// From and To bound the note date.
	From Date `json:"from"`
	To   Date `json:"to"`
	// OutstandingOnly keeps only the notes with money still owed on them.
	OutstandingOnly bool `json:"outstandingOnly"`
	// OverdueOnly keeps only the notes past their expected delivery date and
	// not yet delivered.
	OverdueOnly bool `json:"overdueOnly"`
}

// Rows lists the notes as the register shows them, newest number first within
// a series and the series in order.
func Rows(notes []Note, rates Rates, carrier Carrier, books []Book, today Date) []Row {
	out := make([]Row, 0, len(notes))
	for _, n := range notes {
		out = append(out, NewRow(n, rates, carrier, WidthOf(books, n.Number.Series), today))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Series != out[j].Series {
			return out[i].Series < out[j].Series
		}
		return out[i].Seq > out[j].Seq
	})
	return out
}

// NewRow turns one note into a register row.
func NewRow(n Note, rates Rates, carrier Carrier, width int, today Date) Row {
	n = n.Normalise()
	totals := Total(n)
	liability := Liable(n)
	tax := TaxOn(n, rates)
	report := Check(n, rates, carrier)
	step, of := n.Status.Stage()

	row := Row{
		Number:     n.Number.Format(width),
		Series:     n.Number.Series,
		Seq:        n.Number.Seq,
		Date:       n.Date,
		DatePrint:  n.Date.Print(),
		Status:     n.Status,
		StatusName: n.Status.Name(),
		Stage:      step,
		StageOf:    of,

		Consignor: n.Consignor.Name,
		Consignee: n.Consignee.Name,
		Route:     n.Route(),
		Goods:     n.GoodsSummary(),

		Packages: totals.Packages,
		Weight:   totals.ActualGrams.String(),

		Terms:      n.Terms.Basis,
		TermsStamp: n.Terms.Basis.Stamp(),

		GrossPaise:       totals.GrossPaise,
		OutstandingPaise: liability.Outstanding,
		DeclaredPaise:    totals.DeclaredValuePaise,

		Vehicle:     n.Vehicle.Printed(),
		Driver:      n.Driver.Name,
		EWayBill:    n.EWayBillNumber,
		TaxPosition: n.Tax.Position.Name(),
		TaxSettled:  tax.Settled,
		TaxPaise:    tax.Amount,

		ExpectedOn:  n.ExpectedDelivery(),
		DeliveredOn: n.Delivery.Date,

		Blockers: report.Blockers,
		Checks:   report.Checks,
		Issuable: report.Issuable,
	}

	// Overdue is measured against today for a consignment still out, and
	// against the delivery date for one that arrived late.
	switch {
	case n.Status == StatusDelivered && !row.ExpectedOn.IsZero() && !n.Delivery.Date.IsZero():
		if late := row.ExpectedOn.DaysUntil(n.Delivery.Date); late > 0 {
			row.DaysLate = late
		}
	case n.Status.Open() && !row.ExpectedOn.IsZero() && !today.IsZero():
		if late := row.ExpectedOn.DaysUntil(today); late > 0 {
			row.DaysLate = late
			row.Overdue = true
		}
	}
	return row
}

// Match reports whether a row passes a filter.
func (f Filter) Match(r Row) bool {
	if series := NormaliseSeries(f.Series); series != "" && r.Series != series {
		return false
	}
	if len(f.Status) > 0 {
		found := false
		for _, want := range f.Status {
			if r.Status == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if !f.From.IsZero() && !r.Date.IsZero() && r.Date.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && !r.Date.IsZero() && r.Date.After(f.To) {
		return false
	}
	if f.OutstandingOnly && r.OutstandingPaise == 0 {
		return false
	}
	if f.OverdueOnly && !r.Overdue {
		return false
	}

	if text := strings.ToLower(strings.TrimSpace(f.Text)); text != "" {
		haystack := strings.ToLower(strings.Join([]string{
			r.Number, r.Consignor, r.Consignee, r.Route, r.Goods, r.Vehicle, r.EWayBill, r.Driver, r.StatusName,
		}, " \x00 "))
		if !strings.Contains(haystack, text) {
			return false
		}
	}
	return true
}

// Select narrows the register.
func Select(rows []Row, f Filter) []Row {
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		if f.Match(r) {
			out = append(out, r)
		}
	}
	return out
}

// Summarise adds the register up.
func Summarise(rows []Row) Summary {
	out := Summary{Notes: len(rows)}
	for _, r := range rows {
		switch r.Status {
		case StatusBooked:
			out.Booked++
		case StatusInTransit:
			out.InTransit++
		case StatusArrived:
			out.Arrived++
		case StatusDelivered:
			out.Delivered++
		case StatusCancelled:
			out.Cancelled++
		case StatusVoid:
			out.Void++
		}
		if r.Status.Open() {
			out.Open++
			if r.Terms == TermsToPay {
				out.ToCollectPaise += r.OutstandingPaise
			}
		}
		if r.Overdue {
			out.Overdue++
		}
		if r.Status != StatusVoid {
			out.GrossPaise += r.GrossPaise
			out.DeclaredPaise += r.DeclaredPaise
		}
		out.OutstandingPaise += r.OutstandingPaise
		if !r.TaxSettled && r.Status != StatusVoid {
			out.TaxUnstated++
		}
		if r.Blockers > 0 {
			out.WithBlockers++
		}
		if r.EWayBill == "" && r.Status != StatusVoid {
			out.WithoutEWayBill++
		}
	}
	return out
}

// Headline is the register's summary in one line.
func (s Summary) Headline() string {
	if s.Notes == 0 {
		return "No consignment notes in the register yet."
	}
	parts := []string{plural(s.Notes, "consignment note", "consignment notes")}
	if s.Open > 0 {
		parts = append(parts, fmt.Sprintf("%d still out", s.Open))
	}
	if s.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("%d past the expected delivery date", s.Overdue))
	}
	if s.ToCollectPaise > 0 {
		parts = append(parts, fmt.Sprintf("₹%s to collect at the door", s.ToCollectPaise.Format()))
	}
	return strings.Join(parts, ", ") + "."
}
