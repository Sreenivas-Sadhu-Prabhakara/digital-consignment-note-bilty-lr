package consignment

import "strconv"

// The register, the serial audit and one note's findings as rows, for an office
// that wants its own copy in a spreadsheet. Money comes out as rupees with two
// places, because that is what a spreadsheet will add up; the paise are exact
// either way, since every figure was a whole number of them to begin with.

// RegisterCSVHeader is the first row of the register file.
func RegisterCSVHeader() []string {
	return []string{
		"Consignment note no.", "Series", "Sequence", "Date", "Status",
		"From", "To", "Consignor", "Consignee", "Goods",
		"Packages", "Weight (kg)", "Freight term",
		"Gross freight (INR)", "Outstanding (INR)", "Declared value (INR)",
		"Tax position", "Tax stated", "Tax on carriage (INR)",
		"Vehicle", "Driver", "E-way bill no.",
		"Expected at destination", "Delivered on", "Days late",
		"Blockers", "Checks",
	}
}

// RegisterCSVRows is the register as rows.
func RegisterCSVRows(rows []Row) [][]string {
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, []string{
			r.Number,
			r.Series,
			strconv.Itoa(r.Seq),
			r.Date.String(),
			r.StatusName,
			"", // From and To are in the route; filled below.
			"",
			r.Consignor,
			r.Consignee,
			r.Goods,
			strconv.Itoa(r.Packages),
			r.Weight,
			string(r.Terms),
			r.GrossPaise.Format(),
			r.OutstandingPaise.Format(),
			r.DeclaredPaise.Format(),
			r.TaxPosition,
			yesNo(r.TaxSettled),
			r.TaxPaise.Format(),
			r.Vehicle,
			r.Driver,
			r.EWayBill,
			r.ExpectedOn.String(),
			r.DeliveredOn.String(),
			strconv.Itoa(r.DaysLate),
			strconv.Itoa(r.Blockers),
			strconv.Itoa(r.Checks),
		})
	}
	return out
}

// RegisterCSVRowsFrom is the register as rows with the route split back into
// its two places, which a spreadsheet sorts on better than a joined string.
func RegisterCSVRowsFrom(notes []Note, rows []Row) [][]string {
	places := map[string][2]string{}
	for _, n := range notes {
		n = n.Normalise()
		places[n.Number.Key()] = [2]string{n.From, n.To}
	}

	out := RegisterCSVRows(rows)
	for i, r := range rows {
		if place, ok := places[Number{Series: r.Series, Seq: r.Seq}.Key()]; ok {
			out[i][5] = place[0]
			out[i][6] = place[1]
		}
	}
	return out
}

// AuditCSVHeader is the first row of the serial audit file.
func AuditCSVHeader() []string {
	return []string{
		"Series", "Label", "First", "Last", "Numbers in run", "Accounted for",
		"Missing", "Issued", "Delivered", "Cancelled", "Void",
		"Intact", "Next number", "Verdict",
	}
}

// AuditCSVRows is the serial audit as rows, one per series, followed by a row
// for each break so the file carries the breaks and not just the counts.
func AuditCSVRows(a RegisterAudit) [][]string {
	out := make([][]string, 0, len(a.Series)*2)
	for _, s := range a.Series {
		out = append(out, []string{
			s.Series, s.Label,
			strconv.Itoa(s.First), strconv.Itoa(s.Last),
			strconv.Itoa(s.Span), strconv.Itoa(s.Accounted), strconv.Itoa(s.Missing),
			strconv.Itoa(s.Issued), strconv.Itoa(s.Delivered),
			strconv.Itoa(s.Cancelled), strconv.Itoa(s.Void),
			yesNo(s.Intact), strconv.Itoa(s.Next), s.Verdict,
		})
	}
	for _, s := range a.Series {
		for _, g := range s.Gaps {
			out = append(out, []string{s.Series, s.Label, "", "", "", "", strconv.Itoa(g.Count),
				"", "", "", "", "no", "", "GAP: " + g.Printed + " unaccounted for"})
		}
		for _, r := range s.Reuses {
			out = append(out, []string{s.Series, s.Label, "", "", "", strconv.Itoa(r.Count), "",
				"", "", "", "", "no", "", "REUSE: " + r.Printed + " is on " + strconv.Itoa(r.Count) + " notes"})
		}
		for _, o := range s.OutOfOrder {
			out = append(out, []string{s.Series, s.Label, "", "", "", "", "",
				"", "", "", "", "", "", "OUT OF ORDER: " + o.Printed})
		}
	}
	return out
}

// FindingCSVHeader is the first row of the findings file.
func FindingCSVHeader() []string {
	return []string{"Severity", "Particular", "What", "What to do"}
}

// FindingCSVRows is one note's findings as rows.
func FindingCSVRows(findings []Finding) [][]string {
	out := make([][]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, []string{string(f.Severity), f.Field, f.What, f.Fix})
	}
	return out
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
