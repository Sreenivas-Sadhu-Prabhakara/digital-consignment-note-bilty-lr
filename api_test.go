package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/digital-consignment-note-bilty-lr/internal/consignment"
)

// serve stands the interface up over a register in a temporary directory.
func serve(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := openStore(filepath.Join(t.TempDir(), "register.json"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}

	mux := http.NewServeMux()
	(&api{store: st}).routes(mux)
	srv := httptest.NewServer(guard(mux))
	t.Cleanup(srv.Close)
	return srv
}

// call makes a request and decodes the answer into out, returning the status.
func call(t *testing.T, srv *httptest.Server, method, path string, body any, out any) int {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshalling the request: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s answered with something that is not JSON: %v", method, path, err)
		}
	}
	return res.StatusCode
}

type problem struct {
	Error    string   `json:"error"`
	Problems []string `json:"problems"`
}

func TestGetStateOnAnEmptyRegister(t *testing.T) {
	srv := serve(t)

	var got statePayload
	if code := call(t, srv, "GET", "/api/state", nil, &got); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(got.Rows) != 0 {
		t.Errorf("rows = %d, want none", len(got.Rows))
	}
	if !got.Audit.Intact {
		t.Errorf("an empty register is broken: %s", got.Audit.Headline)
	}
	if got.Today.IsZero() {
		t.Error("the state does not say what day it is")
	}
	if got.Rates != consignment.DefaultRates() {
		t.Errorf("rates = %+v, want the shipped figures", got.Rates)
	}
	if len(got.Constants) < 5 {
		t.Errorf("only %d constants are published to the page", len(got.Constants))
	}
	if len(got.Reference.Statuses) != len(consignment.Statuses()) {
		t.Errorf("the page is offered %d statuses, want %d", len(got.Reference.Statuses), len(consignment.Statuses()))
	}
	if len(got.Reference.Positions) != len(consignment.Positions()) {
		t.Errorf("the page is offered %d tax positions, want %d", len(got.Reference.Positions), len(consignment.Positions()))
	}
}

func TestTheWholeRunThroughTheInterface(t *testing.T) {
	srv := serve(t)

	// The office's own details.
	carrier := consignment.Carrier{
		Name: "Deccan Roadlines", Address: "Shed 14", City: "Bengaluru",
		State: "Karnataka", PIN: "560022", PAN: "AABCD1234E",
	}
	var state statePayload
	if code := call(t, srv, "PUT", "/api/carrier", carrier, &state); code != http.StatusOK {
		t.Fatalf("PUT /api/carrier = %d", code)
	}
	if state.Carrier.Name != "Deccan Roadlines" {
		t.Fatalf("the carrier was not saved: %+v", state.Carrier)
	}

	// A numbered series.
	if code := call(t, srv, "PUT", "/api/book",
		consignment.Book{Series: "BLR", Label: "Bengaluru booking office", SeqWidth: 4, StartAt: 1}, &state); code != http.StatusOK {
		t.Fatalf("PUT /api/book = %d", code)
	}

	// A new note gets the first number.
	var note notePayload
	if code := call(t, srv, "POST", "/api/notes?series=BLR", nil, &note); code != http.StatusCreated {
		t.Fatalf("POST /api/notes = %d", code)
	}
	if note.Number != "BLR/0001" {
		t.Fatalf("the first note is numbered %q", note.Number)
	}
	if note.Tax.Settled {
		t.Error("a new note already has a tax position settled")
	}
	if note.Report.Issuable {
		t.Error("an empty new note reported itself issuable")
	}

	// Fill it in.
	filled := note.Note
	filled.From = "Bengaluru"
	filled.To = "Hubballi"
	filled.Consignor = consignment.Party{Name: "Sahyadri Hardware", GSTIN: "29AAACS1234J1ZF"}
	filled.Consignee = consignment.Party{Name: "Malnad Supplies", GSTIN: "29AAFCM4567K1ZW"}
	filled.Goods = []consignment.Goods{
		{Description: "Galvanised iron pipes", Packages: 40, Packing: "Bundles", ActualGrams: 4800 * consignment.GramsPerKilogram},
		{Description: "Pipe fittings", Packages: 12, Packing: "Cartons", ActualGrams: 360 * consignment.GramsPerKilogram},
	}
	filled.DeclaredValuePaise = 418000 * consignment.PaisePerRupee
	filled.ValueDeclaredByName = "Sahyadri Hardware"
	filled.Terms = consignment.Terms{Basis: consignment.TermsToPay}
	filled.Charges = consignment.Charges{Basis: consignment.BasisPerQuintal, RatePaise: 185 * consignment.PaisePerRupee, LoadingPaise: 90000}
	filled.Vehicle = consignment.Vehicle{Number: "KA25C4471", Kind: "1109 / 10-tyre"}
	filled.Driver = consignment.Driver{Name: "Mahadev Kamble", LicenceNo: "KA2520180004471"}
	filled.Tax = consignment.TaxStated{Position: consignment.PositionRecipientReverseCharge}
	filled.EWayBillNumber = "381004117762"
	filled.TransitDays = 2

	if code := call(t, srv, "PUT", "/api/note", filled, &note); code != http.StatusOK {
		t.Fatalf("PUT /api/note = %d", code)
	}
	if note.Totals.Packages != 52 {
		t.Errorf("packages = %d, want 52", note.Totals.Packages)
	}
	if want := consignment.Paise(1044600); note.Totals.GrossPaise != want {
		t.Errorf("gross = ₹%s, want ₹%s", note.Totals.GrossPaise.Format(), want.Format())
	}
	if !note.Report.Issuable {
		t.Errorf("the filled note is not issuable: %+v", note.Report.Findings)
	}
	if !note.Tax.Settled || note.Tax.Amount == 0 {
		t.Errorf("the tax position did not come out: %+v", note.Tax)
	}
	if len(note.Tax.Caveats) == 0 {
		t.Error("a tax figure came back with no caveats")
	}
	if note.Expected.IsZero() {
		t.Error("no expected delivery date was worked out")
	}

	// Out on the road.
	if code := call(t, srv, "POST", "/api/note/status?number=BLR/0001&to=in-transit", nil, &note); code != http.StatusOK {
		t.Fatalf("POST /api/note/status = %d", code)
	}
	if note.Note.Status != consignment.StatusInTransit {
		t.Errorf("status = %s", note.Note.Status)
	}

	// Going backwards is refused.
	var prob problem
	if code := call(t, srv, "POST", "/api/note/status?number=BLR/0001&to=booked", nil, &prob); code != http.StatusConflict {
		t.Errorf("going back to booked gave %d, want a conflict", code)
	}

	// The copy set before delivery has three copies, not four.
	var copies struct {
		Set consignment.CopySet `json:"set"`
	}
	if code := call(t, srv, "GET", "/api/copies?number=BLR/0001", nil, &copies); code != http.StatusOK {
		t.Fatalf("GET /api/copies = %d", code)
	}
	if len(copies.Set.Copies) != 3 {
		t.Errorf("got %d copies before delivery, want three", len(copies.Set.Copies))
	}
	if _, withheld := copies.Set.Withheld["proof-of-delivery"]; !withheld {
		t.Error("the proof of delivery was handed over before the consignment was delivered")
	}

	// Delivered. The note was dated the day it was raised, so the delivery is
	// dated from that and not from a date written into the test.
	record := consignment.Delivery{
		Date: note.Note.Date.AddDays(2), ReceiverName: "Shivanand Patil",
		ReceiverRole: "godown keeper", PackagesReceived: 52, PackagesCounted: true,
		Condition: consignment.ConditionGoodOrder, FreightCollectedPaise: 1044600,
	}
	if code := call(t, srv, "POST", "/api/note/delivery?number=BLR/0001", record, &note); code != http.StatusOK {
		t.Fatalf("POST /api/note/delivery = %d", code)
	}
	if note.Note.Status != consignment.StatusDelivered {
		t.Errorf("status = %s, want delivered", note.Note.Status)
	}
	if note.Liable.Outstanding != 0 {
		t.Errorf("outstanding = ₹%s after the freight was collected", note.Liable.Outstanding.Format())
	}

	// And now there are four copies, the fourth carrying the record.
	if code := call(t, srv, "GET", "/api/copies?number=BLR/0001", nil, &copies); code != http.StatusOK {
		t.Fatalf("GET /api/copies = %d", code)
	}
	if len(copies.Set.Copies) != 4 {
		t.Fatalf("got %d copies after delivery, want four", len(copies.Set.Copies))
	}
	pod := copies.Set.Copies[3]
	if pod.Kind != consignment.CopyProofOfDelivery {
		t.Fatalf("the fourth copy is %s", pod.Kind)
	}
	if pod.POD == nil || pod.POD.ReceivedBy != "Shivanand Patil" {
		t.Errorf("the proof of delivery does not carry the record: %+v", pod.POD)
	}
	if !pod.POD.Clean {
		t.Error("a complete delivery produced a qualified proof of delivery")
	}

	// The register now holds one delivered note and an intact series.
	if code := call(t, srv, "GET", "/api/state", nil, &state); code != http.StatusOK {
		t.Fatalf("GET /api/state = %d", code)
	}
	if len(state.Rows) != 1 || state.Rows[0].Number != "BLR/0001" {
		t.Errorf("rows = %+v", state.Rows)
	}
	if state.Summary.Delivered != 1 {
		t.Errorf("delivered = %d, want 1", state.Summary.Delivered)
	}
	if !state.Audit.Intact {
		t.Errorf("the series is broken: %s", state.Audit.Headline)
	}
}

func TestTheInterfaceRefusesToDeleteANote(t *testing.T) {
	srv := serve(t)
	var note notePayload
	if code := call(t, srv, "POST", "/api/notes?series=BLR", nil, &note); code != http.StatusCreated {
		t.Fatalf("POST /api/notes = %d", code)
	}

	var prob problem
	code := call(t, srv, "DELETE", "/api/note?number=BLR/0001", nil, &prob)
	if code != http.StatusConflict {
		t.Fatalf("DELETE /api/note = %d, want a conflict", code)
	}
	for _, want := range []string{"keeps its number", "hole in the series", "Cancel it", "void"} {
		if !strings.Contains(prob.Error, want) {
			t.Errorf("the refusal does not mention %q: %s", want, prob.Error)
		}
	}

	// And the note is still there.
	var state statePayload
	call(t, srv, "GET", "/api/state", nil, &state)
	if len(state.Rows) != 1 {
		t.Errorf("rows = %d after a refused delete, want 1", len(state.Rows))
	}
}

func TestTheInterfaceReportsABrokenSeriesLoudly(t *testing.T) {
	srv := serve(t)

	// Three notes, then move the third forward to make a hole.
	for i := 0; i < 3; i++ {
		var note notePayload
		if code := call(t, srv, "POST", "/api/notes?series=BLR", nil, &note); code != http.StatusCreated {
			t.Fatalf("POST /api/notes = %d", code)
		}
	}
	var note notePayload
	if code := call(t, srv, "POST", "/api/note/renumber?number=BLR/0003&to=7", nil, &note); code != http.StatusOK {
		t.Fatalf("POST /api/note/renumber = %d", code)
	}

	var state statePayload
	call(t, srv, "GET", "/api/state", nil, &state)
	if state.Audit.Intact {
		t.Fatal("the register reported an intact series with a hole in it")
	}
	if !strings.Contains(state.Audit.Headline, "BROKEN") {
		t.Errorf("headline = %q, want it to say so loudly", state.Audit.Headline)
	}
	// The run now goes 1, 2, 7, so numbers 3 to 6 are unaccounted for.
	if state.Audit.Missing != 4 {
		t.Errorf("missing = %d, want 4 (numbers 3 to 6): %s", state.Audit.Missing, state.Audit.Headline)
	}
	if len(state.Audit.Breaks) == 0 {
		t.Error("the register names no breaks")
	}

	// Account for the holes and it comes right.
	for _, seq := range []string{"4", "5", "6"} {
		if code := call(t, srv, "POST", "/api/void?series=BLR&seq="+seq,
			consignment.Cancellation{Reason: "leaf spoiled at the counter"}, &note); code != http.StatusCreated {
			t.Fatalf("POST /api/void seq=%s = %d", seq, code)
		}
	}
	call(t, srv, "GET", "/api/state", nil, &state)
	if state.Audit.Missing != 1 {
		t.Errorf("missing = %d, want 1 (number 3 is still unaccounted for): %s", state.Audit.Missing, state.Audit.Headline)
	}
	if code := call(t, srv, "POST", "/api/void?series=BLR&seq=3",
		consignment.Cancellation{Reason: "number skipped at the counter"}, &note); code != http.StatusCreated {
		t.Fatalf("POST /api/void = %d", code)
	}
	call(t, srv, "GET", "/api/state", nil, &state)
	if !state.Audit.Intact {
		t.Errorf("the series is still broken: %s\n%v", state.Audit.Headline, state.Audit.Breaks)
	}
}

func TestTheInterfaceRefusesAVoidOverANumberInUse(t *testing.T) {
	srv := serve(t)
	var note notePayload
	call(t, srv, "POST", "/api/notes?series=BLR", nil, &note)

	var prob problem
	code := call(t, srv, "POST", "/api/void?number=BLR/0001",
		consignment.Cancellation{Reason: "leaf spoiled"}, &prob)
	if code != http.StatusConflict {
		t.Fatalf("status = %d, want a conflict", code)
	}
	if !strings.Contains(prob.Error, "cancelled, not voided") {
		t.Errorf("the refusal does not say what to do instead: %s", prob.Error)
	}
}

func TestTheInterfaceLoadsAndClearsTheWorkedRegister(t *testing.T) {
	srv := serve(t)

	var state statePayload
	if code := call(t, srv, "POST", "/api/sample", nil, &state); code != http.StatusOK {
		t.Fatalf("POST /api/sample = %d", code)
	}
	if len(state.Rows) != 5 {
		t.Fatalf("got %d rows in the worked register, want 5", len(state.Rows))
	}
	if !state.Audit.Intact {
		t.Errorf("the worked register is broken: %s", state.Audit.Headline)
	}
	if state.Summary.WithBlockers != 0 {
		t.Errorf("%d notes in the worked register are not ready to issue", state.Summary.WithBlockers)
	}
	if state.Carrier.Name == "" {
		t.Error("the worked register has no carrier")
	}

	if code := call(t, srv, "POST", "/api/clear", nil, &state); code != http.StatusOK {
		t.Fatalf("POST /api/clear = %d", code)
	}
	if len(state.Rows) != 0 {
		t.Errorf("rows = %d after clearing", len(state.Rows))
	}
	if state.Carrier.Name == "" {
		t.Error("clearing the register lost the office's own details")
	}
}

func TestTheInterfaceSavesChangedRates(t *testing.T) {
	// The whole point of keeping the rates in the file is that a change to the
	// law is a change to a number on screen.
	srv := serve(t)
	call(t, srv, "POST", "/api/sample", nil, nil)

	next := consignment.DefaultRates()
	next.GTAForwardWithITC = 1800

	var state statePayload
	if code := call(t, srv, "PUT", "/api/rates", next, &state); code != http.StatusOK {
		t.Fatalf("PUT /api/rates = %d", code)
	}
	if state.Rates.GTAForwardWithITC != 1800 {
		t.Errorf("the rate came back as %s", state.Rates.GTAForwardWithITC)
	}

	// And it is reflected in the published constants and in a note's figure.
	found := false
	for _, c := range state.Constants {
		if c.Name == "RateGTAForwardWithITCDefault" {
			found = true
			if c.Value != "18%" {
				t.Errorf("the published constant says %q", c.Value)
			}
		}
	}
	if !found {
		t.Error("the changed rate is not published to the page")
	}

	var note notePayload
	call(t, srv, "GET", "/api/note?number=BLR/0003", nil, &note)
	if note.Tax.Rate != 1800 {
		t.Errorf("the note's tax rate is %s, want the register's changed 18%%", note.Tax.Rate)
	}
}

func TestTheInterfaceAnswersWithProblemsItCanShow(t *testing.T) {
	srv := serve(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"a note that is not there", "GET", "/api/note?number=BLR/0099", nil, http.StatusNotFound},
		{"copies of a note that is not there", "GET", "/api/copies?number=BLR/0099", nil, http.StatusNotFound},
		{"findings for a note that is not there", "GET", "/api/findings.csv?number=BLR/0099", nil, http.StatusNotFound},
		{"no number at all", "GET", "/api/note", nil, http.StatusBadRequest},
		{"a number that is not one", "GET", "/api/note?number=HUB-NORTH", nil, http.StatusBadRequest},
		{"a sequence that is not a number", "GET", "/api/note?series=BLR&seq=abc", nil, http.StatusBadRequest},
		{"no status to move to", "POST", "/api/note/status?number=BLR/0001", nil, http.StatusBadRequest},
		{"no number to renumber to", "POST", "/api/note/renumber?number=BLR/0001", nil, http.StatusBadRequest},
		{"a body that is not JSON", "PUT", "/api/carrier", nil, http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var prob problem
			code := call(t, srv, c.method, c.path, c.body, &prob)
			if code != c.want {
				t.Errorf("status = %d, want %d (answer: %s)", code, c.want, prob.Error)
			}
			if prob.Error == "" {
				t.Error("the answer carries no message the page can show")
			}
		})
	}
}

func TestTheInterfaceRefusesUnknownFieldsInABody(t *testing.T) {
	srv := serve(t)
	req := strings.NewReader(`{"name":"Deccan Roadlines","somethingElse":1}`)

	res, err := srv.Client().Post(srv.URL+"/api/carrier", "application/json", req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	// POST is not the method the route takes, so the mux answers first; what
	// matters is that it is not a success.
	if res.StatusCode == http.StatusOK {
		t.Error("a body with an unknown field was accepted")
	}
}

func TestTheInterfaceRefusesARequestFromAnotherSite(t *testing.T) {
	srv := serve(t)

	req, err := http.NewRequest("GET", srv.URL+"/api/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://somewhere-else.example")

	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want it refused", res.StatusCode)
	}
}

func TestTheInterfaceSetsTheHeadersALocalToolNeeds(t *testing.T) {
	srv := serve(t)
	res, err := srv.Client().Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := res.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestTheCSVFiles(t *testing.T) {
	srv := serve(t)
	call(t, srv, "POST", "/api/sample", nil, nil)

	cases := []struct {
		name     string
		path     string
		filename string
		minRows  int
	}{
		{"the register", "/api/register.csv", "consignment-note-register.csv", 6},
		{"the serial audit", "/api/audit.csv", "serial-audit.csv", 2},
		{"one note's findings", "/api/findings.csv?number=BLR/0004", "findings.csv", 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := srv.Client().Get(srv.URL + c.path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", res.StatusCode)
			}
			if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
				t.Errorf("Content-Type = %q", ct)
			}
			if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, c.filename) {
				t.Errorf("Content-Disposition = %q, want it to name %s", cd, c.filename)
			}

			rows, err := csv.NewReader(res.Body).ReadAll()
			if err != nil {
				t.Fatalf("the file does not read as CSV: %v", err)
			}
			if len(rows) < c.minRows {
				t.Errorf("got %d rows, want at least %d", len(rows), c.minRows)
			}
			// Every row has the same number of cells as the heading.
			for i, row := range rows {
				if len(row) != len(rows[0]) {
					t.Errorf("row %d has %d cells against %d headings", i, len(row), len(rows[0]))
				}
			}
		})
	}
}

func TestGetAudit(t *testing.T) {
	srv := serve(t)
	call(t, srv, "POST", "/api/sample", nil, nil)

	var got consignment.RegisterAudit
	if code := call(t, srv, "GET", "/api/audit", nil, &got); code != http.StatusOK {
		t.Fatalf("GET /api/audit = %d", code)
	}
	if !got.Intact {
		t.Errorf("the worked register is broken: %s", got.Headline)
	}
	if len(got.Series) != 1 {
		t.Errorf("got %d series, want 1", len(got.Series))
	}
	if got.Accounted != 5 {
		t.Errorf("accounted for = %d, want 5", got.Accounted)
	}
}

func TestCancelThroughTheInterfaceKeepsTheNumber(t *testing.T) {
	srv := serve(t)
	var note notePayload
	call(t, srv, "POST", "/api/notes?series=BLR", nil, &note)

	var prob problem
	if code := call(t, srv, "POST", "/api/note/cancel?number=BLR/0001",
		consignment.Cancellation{}, &prob); code != http.StatusConflict {
		t.Errorf("cancelling with no reason gave %d, want a conflict", code)
	}

	if code := call(t, srv, "POST", "/api/note/cancel?number=BLR/0001",
		consignment.Cancellation{Date: consignment.Today(), Reason: "consignor withdrew"}, &note); code != http.StatusOK {
		t.Fatalf("POST /api/note/cancel = %d", code)
	}
	if note.Note.Status != consignment.StatusCancelled {
		t.Errorf("status = %s", note.Note.Status)
	}
	if note.Number != "BLR/0001" {
		t.Errorf("the number changed to %q", note.Number)
	}

	// The next note still gets 2.
	call(t, srv, "POST", "/api/notes?series=BLR", nil, &note)
	if note.Number != "BLR/0002" {
		t.Errorf("the next number is %q, want BLR/0002", note.Number)
	}
}
