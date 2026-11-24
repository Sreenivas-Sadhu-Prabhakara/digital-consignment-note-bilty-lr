package consignment

import (
	"strings"
	"testing"
	"time"
)

func sampleRows(t *testing.T) ([]Note, []Row) {
	t.Helper()
	notes := SampleNotes()
	rows := Rows(notes, SampleRates(), SampleCarrier(), []Book{SampleBook()}, MustDate(2026, time.April, 12))
	if len(rows) != len(notes) {
		t.Fatalf("got %d rows for %d notes", len(rows), len(notes))
	}
	return notes, rows
}

func TestRowsAreSortedNewestNumberFirstWithinASeries(t *testing.T) {
	_, rows := sampleRows(t)
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Series == rows[i].Series && rows[i-1].Seq < rows[i].Seq {
			t.Errorf("row %d (%s) sorts before row %d (%s)", i-1, rows[i-1].Number, i, rows[i].Number)
		}
	}
	if rows[0].Number != "BLR/0005" {
		t.Errorf("the first row is %q, want the highest number BLR/0005", rows[0].Number)
	}
}

func TestRowsAcrossSeries(t *testing.T) {
	notes := []Note{
		at("HUB", 2, 1, StatusBooked),
		at("BLR", 1, 2, StatusBooked),
		at("HUB", 1, 3, StatusBooked),
		at("BLR", 2, 4, StatusBooked),
	}
	rows := Rows(notes, DefaultRates(), testCarrier(), nil, Today())
	want := []string{"BLR/0002", "BLR/0001", "HUB/0002", "HUB/0001"}
	for i := range want {
		if rows[i].Number != want[i] {
			t.Errorf("row %d = %q, want %q", i, rows[i].Number, want[i])
		}
	}
}

func TestNewRowCarriesTheDerivedFigures(t *testing.T) {
	notes := SampleNotes()
	// The delivered prepaid note.
	got := NewRow(notes[0], SampleRates(), SampleCarrier(), 4, MustDate(2026, time.April, 12))

	if got.Number != "BLR/0001" {
		t.Errorf("number = %q", got.Number)
	}
	if got.Packages != 52 {
		t.Errorf("packages = %d, want 52", got.Packages)
	}
	if got.Status != StatusDelivered {
		t.Errorf("status = %s", got.Status)
	}
	if step, of := got.Stage, got.StageOf; step != 4 || of != 4 {
		t.Errorf("stage = %d of %d, want 4 of 4", step, of)
	}
	if got.Route != "Bengaluru to Hubballi" {
		t.Errorf("route = %q", got.Route)
	}
	if got.TermsStamp != "FREIGHT PAID" {
		t.Errorf("terms stamp = %q", got.TermsStamp)
	}
	if !got.TaxSettled {
		t.Error("the tax position on the sample note is unsettled")
	}
	if got.Vehicle != "KA 25 C 4471" {
		t.Errorf("vehicle = %q, want it spaced", got.Vehicle)
	}
	if got.Overdue {
		t.Error("a delivered note is overdue")
	}
	if got.DeliveredOn.IsZero() {
		t.Error("the delivery date did not come through")
	}
}

func TestRowOverdueIsMeasuredAgainstToday(t *testing.T) {
	n := carried() // booked 6 April, two days' transit, still in transit
	cases := []struct {
		name     string
		today    Date
		overdue  bool
		daysLate int
	}{
		{"before the expected date", MustDate(2026, time.April, 7), false, 0},
		{"on the expected date", MustDate(2026, time.April, 8), false, 0},
		{"a day past it", MustDate(2026, time.April, 9), true, 1},
		{"a week past it", MustDate(2026, time.April, 15), true, 7},
		{"with no idea what day it is", Date{}, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NewRow(n, DefaultRates(), testCarrier(), 4, c.today)
			if got.Overdue != c.overdue {
				t.Errorf("overdue = %v, want %v", got.Overdue, c.overdue)
			}
			if got.DaysLate != c.daysLate {
				t.Errorf("days late = %d, want %d", got.DaysLate, c.daysLate)
			}
		})
	}
}

func TestRowLatenessOnADeliveredNoteIsMeasuredAgainstTheDeliveryDate(t *testing.T) {
	n := carried()
	n.Status = StatusDelivered
	n.Delivery = Delivery{
		Date: MustDate(2026, time.April, 11), ReceiverName: "A Receiver",
		PackagesReceived: 52, PackagesCounted: true, Condition: ConditionGoodOrder,
	}

	// Expected on 8 April; delivered on the 11th. Today is irrelevant.
	got := NewRow(n, DefaultRates(), testCarrier(), 4, MustDate(2026, time.June, 1))
	if got.DaysLate != 3 {
		t.Errorf("days late = %d, want 3", got.DaysLate)
	}
	if got.Overdue {
		t.Error("a delivered note is still marked overdue")
	}
}

func TestFilterMatch(t *testing.T) {
	_, rows := sampleRows(t)

	cases := []struct {
		name   string
		filter Filter
		want   int
	}{
		{"nothing filters nothing", Filter{}, 5},
		{"by series", Filter{Series: "BLR"}, 5},
		{"by a series with nothing in it", Filter{Series: "HUB"}, 0},
		{"by series, case-insensitively", Filter{Series: "blr"}, 5},
		{"by one status", Filter{Status: []Status{StatusDelivered}}, 1},
		{"by several statuses", Filter{Status: []Status{StatusDelivered, StatusInTransit}}, 2},
		{"by a status nothing is at", Filter{Status: []Status{Status("lost")}}, 0},
		{"by the consignee's name", Filter{Text: "Malnad"}, 1},
		{"by the consignor's name", Filter{Text: "Sahyadri"}, 2},
		{"by a word in the goods", Filter{Text: "groundnut"}, 1},
		{"by the vehicle", Filter{Text: "KA 20"}, 1},
		{"by the e-way bill number", Filter{Text: "381004118039"}, 1},
		{"by the note number", Filter{Text: "BLR/0003"}, 1},
		{"by a word nothing has", Filter{Text: "aeroplane"}, 0},
		{"from a date", Filter{From: MustDate(2026, time.April, 9)}, 3},
		{"to a date", Filter{To: MustDate(2026, time.April, 7)}, 2},
		{"between two dates", Filter{From: MustDate(2026, time.April, 7), To: MustDate(2026, time.April, 9)}, 2},
		// The delivered note was paid in full at booking, so the three with
		// money still owed are the to-pay one, the third-party one and nothing
		// else; the cancelled and void numbers owe nothing by construction.
		{"outstanding only", Filter{OutstandingOnly: true}, 2},
		{"overdue only", Filter{OverdueOnly: true}, 2},
		{"two filters at once", Filter{Series: "BLR", Status: []Status{StatusInTransit}}, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Select(rows, c.filter)
			if len(got) != c.want {
				var numbers []string
				for _, r := range got {
					numbers = append(numbers, r.Number+"/"+r.StatusName)
				}
				t.Errorf("Select gave %d rows (%s), want %d", len(got), strings.Join(numbers, ", "), c.want)
			}
		})
	}
}

func TestFilterTextIsCaseInsensitive(t *testing.T) {
	_, rows := sampleRows(t)
	lower := Select(rows, Filter{Text: "malnad"})
	upper := Select(rows, Filter{Text: "MALNAD"})
	if len(lower) != 1 || len(upper) != 1 {
		t.Errorf("case changed the result: %d against %d", len(lower), len(upper))
	}
}

func TestSummarise(t *testing.T) {
	_, rows := sampleRows(t)
	got := Summarise(rows)

	if got.Notes != 5 {
		t.Errorf("notes = %d, want 5", got.Notes)
	}
	if got.Delivered != 1 || got.InTransit != 1 || got.Arrived != 1 || got.Cancelled != 1 || got.Void != 1 {
		t.Errorf("counts by status = %+v", got)
	}
	if got.Open != 2 {
		t.Errorf("open = %d, want 2 (in transit and arrived)", got.Open)
	}

	// Only the to-pay note still out contributes to what the clerk collects.
	if got.ToCollectPaise == 0 {
		t.Error("nothing to collect at the door, but a to-pay note is still out")
	}
	if got.WithBlockers != 0 {
		t.Errorf("%d notes in the worked register have blockers", got.WithBlockers)
	}
	// Two notes in the worked register have no position stated: the cancelled
	// one and the void one. The void one is not counted.
	if got.TaxUnstated != 1 {
		t.Errorf("tax unstated = %d, want 1 (the cancelled note; the void number is not counted)", got.TaxUnstated)
	}
	if !strings.Contains(got.Headline(), "5 consignment notes") {
		t.Errorf("headline = %q", got.Headline())
	}
	if !strings.Contains(got.Headline(), "to collect at the door") {
		t.Errorf("headline does not mention the money to collect: %q", got.Headline())
	}
}

func TestSummariseLeavesAVoidNumberOutOfTheMoney(t *testing.T) {
	// A voided number carries no consignment, so it contributes nothing to the
	// freight, the declared value or the e-way bill count.
	withVoid := []Note{sampleVoid()}
	rows := Rows(withVoid, DefaultRates(), SampleCarrier(), []Book{SampleBook()}, Today())
	got := Summarise(rows)

	if got.GrossPaise != 0 || got.DeclaredPaise != 0 {
		t.Errorf("a void number contributed money: %+v", got)
	}
	if got.WithoutEWayBill != 0 {
		t.Errorf("a void number was counted as missing an e-way bill number")
	}
	if got.TaxUnstated != 0 {
		t.Error("a void number was counted as having no tax position")
	}
	if got.Void != 1 {
		t.Errorf("void = %d, want 1", got.Void)
	}
}

func TestSummaryHeadlineOnAnEmptyRegister(t *testing.T) {
	got := Summarise(nil)
	if got.Notes != 0 {
		t.Errorf("notes = %d", got.Notes)
	}
	if !strings.Contains(got.Headline(), "No consignment notes") {
		t.Errorf("headline = %q", got.Headline())
	}
}

func TestRegisterCSV(t *testing.T) {
	notes, rows := sampleRows(t)

	header := RegisterCSVHeader()
	body := RegisterCSVRowsFrom(notes, rows)
	if len(body) != len(rows) {
		t.Fatalf("got %d CSV rows for %d register rows", len(body), len(rows))
	}
	for i, row := range body {
		if len(row) != len(header) {
			t.Fatalf("CSV row %d has %d cells against %d headings", i, len(row), len(header))
		}
	}

	// The route comes back as two cells a spreadsheet can sort on.
	first := body[len(body)-1] // BLR/0001, sorted last
	if first[5] != "Bengaluru" || first[6] != "Hubballi" {
		t.Errorf("the route cells are %q and %q, want Bengaluru and Hubballi", first[5], first[6])
	}
	if first[0] != "BLR/0001" {
		t.Errorf("the number cell is %q", first[0])
	}
	if first[13] == "" {
		t.Error("the gross freight cell is empty")
	}
}

func TestAuditCSV(t *testing.T) {
	audit := Audit(SampleNotes(), []Book{SampleBook()})
	header := AuditCSVHeader()
	body := AuditCSVRows(audit)

	if len(body) != 1 {
		t.Fatalf("got %d rows for one intact series, want one", len(body))
	}
	for i, row := range body {
		if len(row) != len(header) {
			t.Fatalf("row %d has %d cells against %d headings", i, len(row), len(header))
		}
	}
	if body[0][11] != "yes" {
		t.Errorf("the intact cell says %q, want yes", body[0][11])
	}

	// A broken series puts a row per break into the file, so the file carries
	// the breaks and not just the counts.
	broken := Audit([]Note{
		at("BLR", 1, 1, StatusBooked),
		at("BLR", 3, 2, StatusBooked),
		at("BLR", 3, 3, StatusBooked),
		at("BLR", 4, 1, StatusBooked),
	}, []Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}})

	brokenRows := AuditCSVRows(broken)
	if len(brokenRows) < 4 {
		t.Fatalf("got %d rows for a series with a gap, a reuse and a date out of order, want at least four", len(brokenRows))
	}
	joined := ""
	for _, row := range brokenRows {
		joined += row[len(row)-1] + "\n"
		if len(row) != len(header) {
			t.Fatalf("a break row has %d cells against %d headings", len(row), len(header))
		}
	}
	for _, want := range []string{"GAP:", "REUSE:", "OUT OF ORDER:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the file does not carry a %q row:\n%s", want, joined)
		}
	}
}

func TestFindingCSV(t *testing.T) {
	report := Check(Note{}, DefaultRates(), Carrier{})
	header := FindingCSVHeader()
	body := FindingCSVRows(report.Findings)

	if len(body) != len(report.Findings) {
		t.Fatalf("got %d rows for %d findings", len(body), len(report.Findings))
	}
	if len(body) == 0 {
		t.Fatal("an empty note produced no findings")
	}
	for i, row := range body {
		if len(row) != len(header) {
			t.Fatalf("row %d has %d cells against %d headings", i, len(row), len(header))
		}
		for j, cell := range row {
			if cell == "" {
				t.Errorf("row %d cell %d (%s) is empty", i, j, header[j])
			}
		}
	}
	if body[0][0] != string(SeverityBlocker) {
		t.Errorf("the first row is a %q, want the blockers first", body[0][0])
	}
}

func TestRegisterCSVRowsFromToleratesANumberItCannotMatch(t *testing.T) {
	// Pass rows built from one register and places from another. The route
	// cells stay empty rather than crashing or picking the wrong note.
	_, rows := sampleRows(t)
	body := RegisterCSVRowsFrom(nil, rows)
	for _, row := range body {
		if row[5] != "" || row[6] != "" {
			t.Errorf("route cells were filled from nothing: %q and %q", row[5], row[6])
		}
	}
}
