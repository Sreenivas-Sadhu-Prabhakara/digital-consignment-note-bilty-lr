package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/digital-consignment-note-bilty-lr/internal/consignment"
)

// api serves the JSON interface the page talks to. Every call is same-origin
// and nothing leaves the machine.
type api struct {
	store *store
}

// routes registers every endpoint.
func (a *api) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", a.getState)

	mux.HandleFunc("PUT /api/carrier", a.putCarrier)
	mux.HandleFunc("PUT /api/rates", a.putRates)
	mux.HandleFunc("PUT /api/book", a.putBook)

	mux.HandleFunc("POST /api/notes", a.postNote)
	mux.HandleFunc("PUT /api/note", a.putNote)
	mux.HandleFunc("GET /api/note", a.getNote)
	mux.HandleFunc("POST /api/note/status", a.postStatus)
	mux.HandleFunc("POST /api/note/delivery", a.postDelivery)
	mux.HandleFunc("POST /api/note/cancel", a.postCancel)
	mux.HandleFunc("POST /api/note/renumber", a.postRenumber)
	mux.HandleFunc("DELETE /api/note", a.deleteNote)

	mux.HandleFunc("POST /api/void", a.postVoid)

	mux.HandleFunc("GET /api/copies", a.getCopies)
	mux.HandleFunc("GET /api/audit", a.getAudit)

	mux.HandleFunc("POST /api/sample", a.postSample)
	mux.HandleFunc("POST /api/clear", a.postClear)

	mux.HandleFunc("GET /api/register.csv", a.getRegisterCSV)
	mux.HandleFunc("GET /api/audit.csv", a.getAuditCSV)
	mux.HandleFunc("GET /api/findings.csv", a.getFindingsCSV)
}

// reference is the lists the page offers, sent from here so the page does not
// keep a second copy of them.
type reference struct {
	Statuses    []labelled `json:"statuses"`
	Terms       []labelled `json:"terms"`
	Bases       []labelled `json:"bases"`
	Positions   []labelled `json:"positions"`
	Conditions  []labelled `json:"conditions"`
	CopyKinds   []labelled `json:"copyKinds"`
	PackingKind []string   `json:"packingKinds"`
	VehicleKind []string   `json:"vehicleKinds"`
}

type labelled struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

func referenceData() reference {
	out := reference{
		PackingKind: consignment.PackingKinds(),
		VehicleKind: consignment.VehicleKinds(),
	}
	for _, s := range consignment.Statuses() {
		out.Statuses = append(out.Statuses, labelled{Value: string(s), Name: s.Name()})
	}
	for _, t := range consignment.TermsBases() {
		out.Terms = append(out.Terms, labelled{Value: string(t), Name: t.Name()})
	}
	for _, b := range consignment.Bases() {
		out.Bases = append(out.Bases, labelled{Value: string(b), Name: b.Name()})
	}
	for _, p := range consignment.Positions() {
		out.Positions = append(out.Positions, labelled{Value: string(p), Name: p.Name()})
	}
	for _, c := range consignment.Conditions() {
		out.Conditions = append(out.Conditions, labelled{Value: string(c), Name: c.Name()})
	}
	for _, k := range consignment.CopyKinds() {
		out.CopyKinds = append(out.CopyKinds, labelled{Value: string(k), Name: string(k)})
	}
	return out
}

// statePayload is everything the register page needs to draw itself.
type statePayload struct {
	Today    consignment.Date          `json:"today"`
	Carrier  consignment.Carrier       `json:"carrier"`
	Books    []consignment.Book        `json:"books"`
	Rates    consignment.Rates         `json:"rates"`
	Rows     []consignment.Row         `json:"rows"`
	Summary  consignment.Summary       `json:"summary"`
	Headline string                    `json:"headline"`
	Audit    consignment.RegisterAudit `json:"audit"`

	Reference reference                 `json:"reference"`
	Constants []consignment.ConstantRef `json:"constants"`
	DataFile  string                    `json:"dataFile"`
}

// getState answers with the whole register.
func (a *api) getState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.state())
}

// state gathers everything the page draws itself from. The audit is run on
// every read rather than cached, because a cached audit is exactly the stale
// second copy this tool exists to do away with.
func (a *api) state() statePayload {
	data := a.store.snapshot()
	today := consignment.Today()
	rows := consignment.Rows(data.Notes, data.Rates, data.Carrier, data.Books, today)
	summary := consignment.Summarise(rows)

	return statePayload{
		Today:     today,
		Carrier:   data.Carrier,
		Books:     data.Books,
		Rates:     data.Rates,
		Rows:      rows,
		Summary:   summary,
		Headline:  summary.Headline(),
		Audit:     consignment.Audit(data.Notes, data.Books),
		Reference: referenceData(),
		Constants: consignment.Constants(data.Rates),
		DataFile:  a.store.path,
	}
}

// notePayload is one note with everything derived from it.
type notePayload struct {
	Note     consignment.Note      `json:"note"`
	Number   string                `json:"number"`
	Totals   consignment.Totals    `json:"totals"`
	Liable   consignment.Liability `json:"liable"`
	Tax      consignment.TaxView   `json:"tax"`
	Report   consignment.Report    `json:"report"`
	Next     []string              `json:"next"`
	Expected consignment.Date      `json:"expectedDelivery"`
	Width    int                   `json:"width"`
}

// getNote answers with one note and everything derived from it.
func (a *api) getNote(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	for _, n := range data.Notes {
		if n.Number.Key() == number.Key() {
			writeJSON(w, http.StatusOK, a.notePayload(n, data))
			return
		}
	}
	writeProblem(w, http.StatusNotFound, fmt.Errorf("there is no note numbered %s in this register",
		number.Format(consignment.WidthOf(data.Books, number.Series))))
}

func (a *api) notePayload(n consignment.Note, data fileData) notePayload {
	width := consignment.WidthOf(data.Books, n.Number.Series)
	out := notePayload{
		Note:     n,
		Number:   n.Number.Format(width),
		Totals:   consignment.Total(n),
		Liable:   consignment.Liable(n),
		Tax:      consignment.TaxOn(n, data.Rates),
		Report:   consignment.Check(n, data.Rates, data.Carrier),
		Expected: n.ExpectedDelivery(),
		Width:    width,
	}
	for _, next := range consignment.Next(n.Status) {
		out.Next = append(out.Next, string(next))
	}
	return out
}

// putCarrier saves the office's own details.
func (a *api) putCarrier(w http.ResponseWriter, r *http.Request) {
	var in consignment.Carrier
	if !readJSON(w, r, &in) {
		return
	}
	if err := a.store.putCarrier(in); err != nil {
		writeProblem(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state())
}

// putRates saves the figures the tax position is worked out on. They live in
// the data file precisely so a change to the law is a change to a number on
// screen and not a change to the program.
func (a *api) putRates(w http.ResponseWriter, r *http.Request) {
	var in consignment.Rates
	if !readJSON(w, r, &in) {
		return
	}
	if err := a.store.putRates(in); err != nil {
		writeProblem(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state())
}

// putBook adds or changes a number series.
func (a *api) putBook(w http.ResponseWriter, r *http.Request) {
	var in consignment.Book
	if !readJSON(w, r, &in) {
		return
	}
	if err := a.store.putBook(in); err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state())
}

// postNote allocates the next number in a series and starts a note on it.
func (a *api) postNote(w http.ResponseWriter, r *http.Request) {
	series := r.URL.Query().Get("series")
	note, err := a.store.newNote(series, consignment.Today())
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.notePayload(note, a.store.snapshot()))
}

// putNote saves a note and answers with it as it now stands, which is how the
// page shows the checks changing as the note is filled in.
func (a *api) putNote(w http.ResponseWriter, r *http.Request) {
	var in consignment.Note
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putNote(in)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.notePayload(saved, a.store.snapshot()))
}

// postStatus moves a note along.
func (a *api) postStatus(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	to := consignment.Status(strings.TrimSpace(r.URL.Query().Get("to")))
	if to == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which status? pass ?to=booked, in-transit, arrived, delivered or cancelled"))
		return
	}

	saved, err := a.store.setStatus(number, to)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.notePayload(saved, a.store.snapshot()))
}

// postDelivery records a delivery and marks the note delivered.
func (a *api) postDelivery(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	var in consignment.Delivery
	if !readJSON(w, r, &in) {
		return
	}

	saved, err := a.store.deliver(number, in)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.notePayload(saved, a.store.snapshot()))
}

// postCancel cancels a note, which keeps its number.
func (a *api) postCancel(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	var in consignment.Cancellation
	if !readJSON(w, r, &in) {
		return
	}

	saved, err := a.store.cancel(number, in)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.notePayload(saved, a.store.snapshot()))
}

// postRenumber moves a note to the end of its series, which is what a reuse is
// put right by.
func (a *api) postRenumber(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	toSeq, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("to")))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, errors.New("which number is it being moved to? pass ?to=<number>"))
		return
	}

	saved, err := a.store.renumber(number, toSeq)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, a.notePayload(saved, a.store.snapshot()))
}

// postVoid accounts for a number that was never used, which is how a gap in the
// series is closed.
func (a *api) postVoid(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	var in consignment.Cancellation
	if !readJSON(w, r, &in) {
		return
	}

	saved, err := a.store.voidNumber(number, in)
	if err != nil {
		writeProblem(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.notePayload(saved, a.store.snapshot()))
}

// deleteNote refuses.
//
// Deleting a note would put a hole in the number series, and a register with a
// hole in it is not a register. The endpoint exists so that the refusal is
// explicit and says what to do instead, rather than being a route that happens
// not to be there.
func (a *api) deleteNote(w http.ResponseWriter, r *http.Request) {
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	data := a.store.snapshot()
	width := consignment.WidthOf(data.Books, number.Series)

	writeProblem(w, http.StatusConflict, fmt.Errorf(
		"%s cannot be deleted. A consignment note keeps its number for good: deleting one would leave a hole in the series, and the register would stop being evidence that nothing is missing. Cancel it if the consignment was called off, or record it as void if the number was never used.",
		number.Format(width)))
}

// getCopies answers with the copy set for one note.
func (a *api) getCopies(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	for _, n := range data.Notes {
		if n.Number.Key() != number.Key() {
			continue
		}
		width := consignment.WidthOf(data.Books, n.Number.Series)
		writeJSON(w, http.StatusOK, map[string]any{
			"set":     consignment.Build(n, data.Rates, data.Carrier, width),
			"note":    a.notePayload(n, data),
			"carrier": data.Carrier,
			"caveats": consignment.TaxOn(n, data.Rates).Caveats,
		})
		return
	}
	writeProblem(w, http.StatusNotFound, fmt.Errorf("there is no note numbered %s in this register",
		number.Format(consignment.WidthOf(data.Books, number.Series))))
}

// getAudit answers with the serial audit on its own, for a page that wants only
// that.
func (a *api) getAudit(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	writeJSON(w, http.StatusOK, consignment.Audit(data.Notes, data.Books))
}

// postSample loads the worked register.
func (a *api) postSample(w http.ResponseWriter, r *http.Request) {
	if err := a.store.loadSample(); err != nil {
		writeProblem(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state())
}

// postClear empties the register, keeping the office's own details.
func (a *api) postClear(w http.ResponseWriter, r *http.Request) {
	if err := a.store.clear(); err != nil {
		writeProblem(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state())
}

// getRegisterCSV answers with the register as a file.
func (a *api) getRegisterCSV(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	rows := consignment.Rows(data.Notes, data.Rates, data.Carrier, data.Books, consignment.Today())
	writeCSV(w, "consignment-note-register.csv", consignment.RegisterCSVHeader(), consignment.RegisterCSVRowsFrom(data.Notes, rows))
}

// getAuditCSV answers with the serial audit as a file, which is the thing an
// office is asked to produce when its numbering is questioned.
func (a *api) getAuditCSV(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	audit := consignment.Audit(data.Notes, data.Books)
	writeCSV(w, "serial-audit.csv", consignment.AuditCSVHeader(), consignment.AuditCSVRows(audit))
}

// getFindingsCSV answers with one note's findings as a file.
func (a *api) getFindingsCSV(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	number, err := numberFrom(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	for _, n := range data.Notes {
		if n.Number.Key() == number.Key() {
			report := consignment.Check(n, data.Rates, data.Carrier)
			writeCSV(w, "findings.csv", consignment.FindingCSVHeader(), consignment.FindingCSVRows(report.Findings))
			return
		}
	}
	writeProblem(w, http.StatusNotFound, fmt.Errorf("there is no note numbered %s in this register",
		number.Format(consignment.WidthOf(data.Books, number.Series))))
}

// numberFrom reads a consignment note number off a request, either as a printed
// number or as a series and a sequence.
func numberFrom(r *http.Request) (consignment.Number, error) {
	q := r.URL.Query()
	if printed := strings.TrimSpace(q.Get("number")); printed != "" {
		return consignment.ParseNumber(printed)
	}

	seq := strings.TrimSpace(q.Get("seq"))
	if seq == "" {
		return consignment.Number{}, errors.New("which note? pass ?number=BLR/0147, or ?series=BLR&seq=147")
	}
	n, err := strconv.Atoi(seq)
	if err != nil {
		return consignment.Number{}, fmt.Errorf("%q is not a consignment note number", seq)
	}
	return consignment.NewNumber(q.Get("series"), n)
}

// readJSON reads a JSON request body, answering with a problem and reporting
// false when it cannot.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeProblem(w, http.StatusBadRequest, fmt.Errorf("this request did not read as JSON: %w", err))
		return false
	}
	return true
}

// writeJSON answers with a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "could not prepare the answer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeProblem answers with an error the page can show as it stands.
func writeProblem(w http.ResponseWriter, status int, err error) {
	problems := strings.Split(err.Error(), "\n")
	writeJSON(w, status, map[string]any{
		"error":    strings.Join(problems, "; "),
		"problems": problems,
	})
}

// writeCSV answers with a CSV file for the browser to save.
func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	out := csv.NewWriter(w)
	if err := out.Write(header); err != nil {
		return
	}
	for _, row := range rows {
		if err := out.Write(row); err != nil {
			return
		}
	}
	out.Flush()
}
