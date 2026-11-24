package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/digital-consignment-note-bilty-lr/internal/consignment"
)

func tempStore(t *testing.T) *store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "register.json")
	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	return s
}

func TestOpenStoreOnNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register.json")

	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("openStore created the file before anything was saved")
	}

	data := s.snapshot()
	if len(data.Notes) != 0 {
		t.Errorf("a new register has %d notes", len(data.Notes))
	}
	// A new register gets the shipped figures, not a rate of nothing.
	if data.Rates != consignment.DefaultRates() {
		t.Errorf("rates = %+v, want the shipped figures %+v", data.Rates, consignment.DefaultRates())
	}
	if len(data.Books) != 1 {
		t.Errorf("a new register has %d books, want one unlabelled series", len(data.Books))
	}
}

func TestOpenStoreOnAnEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(path); err != nil {
		t.Errorf("openStore on an empty file: %v", err)
	}
}

func TestOpenStoreOnRubbish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte("this is not JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := openStore(path)
	if err == nil {
		t.Fatal("openStore read a file that is not JSON")
	}
	if !strings.Contains(err.Error(), "move the file aside") {
		t.Errorf("the error does not say what to do: %v", err)
	}
}

func TestOpenStoreRefusesANewerFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "register.json")
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := openStore(path)
	if err == nil {
		t.Fatal("openStore read a file from a newer version")
	}
	if !strings.Contains(err.Error(), "newer version") {
		t.Errorf("the error does not say why: %v", err)
	}
}

func TestSaveIsAtomicAndPrivate(t *testing.T) {
	s := tempStore(t)
	if _, err := s.newNote("BLR", consignment.Today()); err != nil {
		t.Fatalf("newNote: %v", err)
	}

	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the register is written with mode %o, want 600", perm)
	}

	// No temporary file is left behind.
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
	}

	// And it reads back.
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var back fileData
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the file does not read back as JSON: %v", err)
	}
	if back.Version != fileFormatVersion {
		t.Errorf("version = %d, want %d", back.Version, fileFormatVersion)
	}
	if len(back.Notes) != 1 {
		t.Errorf("notes = %d, want 1", len(back.Notes))
	}
}

func TestStoreRoundTripsThroughTheFile(t *testing.T) {
	s := tempStore(t)
	if err := s.loadSample(); err != nil {
		t.Fatalf("loadSample: %v", err)
	}

	reopened, err := openStore(s.path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	before, after := s.snapshot(), reopened.snapshot()

	if len(after.Notes) != len(before.Notes) {
		t.Fatalf("got %d notes back, wrote %d", len(after.Notes), len(before.Notes))
	}
	for i := range before.Notes {
		if before.Notes[i].Number != after.Notes[i].Number {
			t.Errorf("note %d came back as %v, was %v", i, after.Notes[i].Number, before.Notes[i].Number)
		}
		if before.Notes[i].Date != after.Notes[i].Date {
			t.Errorf("note %d's date came back as %s, was %s", i, after.Notes[i].Date, before.Notes[i].Date)
		}
		if consignment.Total(before.Notes[i]).GrossPaise != consignment.Total(after.Notes[i]).GrossPaise {
			t.Errorf("note %d's gross freight changed across the file", i)
		}
	}
	if after.Carrier.Name != before.Carrier.Name {
		t.Errorf("the carrier came back as %q", after.Carrier.Name)
	}
	if after.Rates != before.Rates {
		t.Errorf("the rates came back as %+v", after.Rates)
	}
}

func TestNewNoteAllocatesSequentiallyAndNeverFillsAGap(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)

	for i := 1; i <= 3; i++ {
		got, err := s.newNote("BLR", today)
		if err != nil {
			t.Fatalf("newNote %d: %v", i, err)
		}
		if got.Number.Seq != i {
			t.Errorf("note %d got number %d", i, got.Number.Seq)
		}
		if got.Status != consignment.StatusBooked {
			t.Errorf("a new note is %s, want booked", got.Status)
		}
		// A new note states no tax position. The tool must not pick one.
		if got.Tax.Position != consignment.PositionUnstated {
			t.Errorf("a new note states the position %s; it must state none", got.Tax.Position)
		}
	}

	// Cancel the middle one. The number stays occupied, so the next allocation
	// still goes to 4.
	if _, err := s.cancel(consignment.Number{Series: "BLR", Seq: 2},
		consignment.Cancellation{Date: today, Reason: "consignor withdrew"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	next, err := s.newNote("BLR", today)
	if err != nil {
		t.Fatalf("newNote: %v", err)
	}
	if next.Number.Seq != 4 {
		t.Errorf("the next number is %d, want 4: a cancelled note keeps its own", next.Number.Seq)
	}

	audit := consignment.Audit(s.snapshot().Notes, s.snapshot().Books)
	if !audit.Intact {
		t.Errorf("the series is broken after a cancellation: %s", audit.Headline)
	}
}

func TestNewNoteStartsFromTheBooksOwnStart(t *testing.T) {
	s := tempStore(t)
	if err := s.putBook(consignment.Book{Series: "HUB", Label: "Hubballi", StartAt: 101}); err != nil {
		t.Fatalf("putBook: %v", err)
	}
	got, err := s.newNote("HUB", consignment.Today())
	if err != nil {
		t.Fatalf("newNote: %v", err)
	}
	if got.Number.Seq != 101 {
		t.Errorf("the first note is numbered %d, want the book's own start of 101", got.Number.Seq)
	}
}

func TestPutBookRefusesToMoveTheStartOfABookInUse(t *testing.T) {
	s := tempStore(t)
	if err := s.putBook(consignment.Book{Series: "BLR", StartAt: 1}); err != nil {
		t.Fatalf("putBook: %v", err)
	}
	if _, err := s.newNote("BLR", consignment.Today()); err != nil {
		t.Fatalf("newNote: %v", err)
	}

	err := s.putBook(consignment.Book{Series: "BLR", StartAt: 50})
	if err == nil {
		t.Fatal("the start of a book in use was moved")
	}
	if !strings.Contains(err.Error(), "first gap") {
		t.Errorf("the refusal does not say why: %v", err)
	}

	// The label and the width can still be changed.
	if err := s.putBook(consignment.Book{Series: "BLR", StartAt: 1, Label: "Bengaluru", SeqWidth: 6}); err != nil {
		t.Errorf("the label could not be changed: %v", err)
	}
	got := bookFor(t, s, "BLR")
	if got.Label != "Bengaluru" || got.SeqWidth != 6 {
		t.Errorf("the book is %+v", got)
	}
}

// bookFor finds a series in the register, failing where it is not declared.
func bookFor(t *testing.T, s *store, series string) consignment.Book {
	t.Helper()
	series = consignment.NormaliseSeries(series)
	for _, b := range s.snapshot().Books {
		if b.Series == series {
			return b
		}
	}
	t.Fatalf("the series %q is not declared in the register: %+v", series, s.snapshot().Books)
	return consignment.Book{}
}

func TestPutNoteCannotChangeTheNumberOrTheStatus(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)
	note, err := s.newNote("BLR", today)
	if err != nil {
		t.Fatalf("newNote: %v", err)
	}

	// Filling the form in works.
	note.From = "Bengaluru"
	note.To = "Hubballi"
	note.Consignor = consignment.Party{Name: "A Consignor"}
	saved, err := s.putNote(note)
	if err != nil {
		t.Fatalf("putNote: %v", err)
	}
	if saved.From != "Bengaluru" {
		t.Errorf("the form was not saved: %+v", saved)
	}

	// Changing the status through the form does not move it.
	sideways := saved
	sideways.Status = consignment.StatusDelivered
	saved, err = s.putNote(sideways)
	if err != nil {
		t.Fatalf("putNote: %v", err)
	}
	if saved.Status != consignment.StatusBooked {
		t.Errorf("the status moved to %s through the form; it has to go through the check", saved.Status)
	}

	// Changing the number through the form is refused: there is no note at the
	// new one.
	renumbered := saved
	renumbered.Number.Seq = 99
	if _, err := s.putNote(renumbered); err == nil {
		t.Error("a note was renumbered through the form")
	}

	// And a note with no number at all is refused.
	unnumbered := saved
	unnumbered.Number.Seq = 0
	if _, err := s.putNote(unnumbered); err == nil {
		t.Error("a note with no number was saved")
	}
}

func TestSetStatusGoesThroughTheCheck(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)
	note, err := s.newNote("BLR", today)
	if err != nil {
		t.Fatal(err)
	}

	// No vehicle yet, so it cannot go in transit.
	if _, err := s.setStatus(note.Number, consignment.StatusInTransit); err == nil {
		t.Error("a consignment went in transit with no vehicle on it")
	}

	note.Vehicle = consignment.Vehicle{Number: "KA01AB1234"}
	if _, err := s.putNote(note); err != nil {
		t.Fatal(err)
	}
	moved, err := s.setStatus(note.Number, consignment.StatusInTransit)
	if err != nil {
		t.Fatalf("setStatus: %v", err)
	}
	if moved.Status != consignment.StatusInTransit {
		t.Errorf("status = %s", moved.Status)
	}

	// Backwards is refused.
	if _, err := s.setStatus(note.Number, consignment.StatusBooked); err == nil {
		t.Error("a consignment went back to booked")
	}

	// A number that is not there is refused.
	if _, err := s.setStatus(consignment.Number{Series: "BLR", Seq: 99}, consignment.StatusInTransit); err == nil {
		t.Error("a status was set on a number that is not in the register")
	}
}

func TestDeliverChecksTheRecordAndMarksTheNote(t *testing.T) {
	s := tempStore(t)
	if err := s.loadSample(); err != nil {
		t.Fatal(err)
	}

	// The to-pay note, number 2, is in transit with 220 packages on it.
	number := consignment.Number{Series: "BLR", Seq: 2}

	// A record with no receiver named is refused.
	if _, err := s.deliver(number, consignment.Delivery{Date: consignment.MustDate(2026, time.April, 9)}); err == nil {
		t.Error("a delivery with nobody named was recorded")
	}

	// Short with the condition still good order is refused.
	_, err := s.deliver(number, consignment.Delivery{
		Date: consignment.MustDate(2026, time.April, 9), ReceiverName: "A Receiver",
		PackagesReceived: 218, PackagesCounted: true, Condition: consignment.ConditionGoodOrder,
	})
	if err == nil {
		t.Error("a short delivery was recorded as good order")
	}

	// A complete record goes through and moves the note.
	got, err := s.deliver(number, consignment.Delivery{
		Date: consignment.MustDate(2026, time.April, 9), ReceiverName: "A Receiver",
		PackagesReceived: 220, PackagesCounted: true, Condition: consignment.ConditionGoodOrder,
		FreightCollectedPaise: 2000000,
	})
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got.Status != consignment.StatusDelivered {
		t.Errorf("status = %s, want delivered", got.Status)
	}
	if got.Delivery.ReceiverName != "A Receiver" {
		t.Errorf("the record was not kept: %+v", got.Delivery)
	}

	// A second record on a delivered note amends the first, because a clerk who
	// typed the receiver's name wrong has to be able to put it right.
	amended := got.Delivery
	amended.ReceiverName = "Shivanand Patil"
	got, err = s.deliver(number, amended)
	if err != nil {
		t.Fatalf("the delivery record could not be amended: %v", err)
	}
	if got.Delivery.ReceiverName != "Shivanand Patil" {
		t.Errorf("the amendment was not kept: %+v", got.Delivery)
	}
	if got.Status != consignment.StatusDelivered {
		t.Errorf("status = %s after an amendment", got.Status)
	}

	// An amendment still goes through the checks.
	bad := amended
	bad.ReceiverName = ""
	if _, err := s.deliver(number, bad); err == nil {
		t.Error("an amendment with nobody named was accepted")
	}
}

func TestCancelKeepsTheNumber(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)
	note, err := s.newNote("BLR", today)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.cancel(note.Number, consignment.Cancellation{Date: today}); err == nil {
		t.Error("a note was cancelled with no reason")
	}

	got, err := s.cancel(note.Number, consignment.Cancellation{Date: today, Reason: "consignor withdrew"})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got.Status != consignment.StatusCancelled {
		t.Errorf("status = %s", got.Status)
	}
	if got.Number.Seq != note.Number.Seq {
		t.Errorf("the number changed from %d to %d", note.Number.Seq, got.Number.Seq)
	}
	if len(s.snapshot().Notes) != 1 {
		t.Error("the note left the register")
	}
}

func TestVoidNumberClosesAGap(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)

	// Two notes, then a hand-made hole by renumbering the second one forward.
	if _, err := s.newNote("BLR", today); err != nil {
		t.Fatal(err)
	}
	second, err := s.newNote("BLR", today)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.renumber(second.Number, 4); err != nil {
		t.Fatalf("renumber: %v", err)
	}

	data := s.snapshot()
	broken := consignment.Audit(data.Notes, data.Books)
	if broken.Intact {
		t.Fatal("the test needs a broken series and did not get one")
	}
	if broken.Missing != 2 {
		t.Fatalf("missing = %d, want 2 (numbers 2 and 3)", broken.Missing)
	}

	// Voiding without a reason is refused.
	if _, err := s.voidNumber(consignment.Number{Series: "BLR", Seq: 2}, consignment.Cancellation{}); err == nil {
		t.Error("a number was voided with no reason")
	}

	// Account for both holes.
	for _, seq := range []int{2, 3} {
		got, err := s.voidNumber(consignment.Number{Series: "BLR", Seq: seq},
			consignment.Cancellation{Date: today, Reason: "leaf spoiled at the counter"})
		if err != nil {
			t.Fatalf("voidNumber %d: %v", seq, err)
		}
		if got.Status != consignment.StatusVoid {
			t.Errorf("number %d is %s, want void", seq, got.Status)
		}
	}

	data = s.snapshot()
	fixed := consignment.Audit(data.Notes, data.Books)
	if !fixed.Intact {
		t.Errorf("the series is still broken after both holes were accounted for: %s\n%v", fixed.Headline, fixed.Breaks)
	}
	series := seriesIn(t, fixed, "BLR")
	if series.Void != 2 {
		t.Errorf("void numbers = %d, want 2", series.Void)
	}
	if series.Accounted != 4 {
		t.Errorf("accounted for = %d, want 4", series.Accounted)
	}
}

// seriesIn finds one series in an audit.
func seriesIn(t *testing.T, a consignment.RegisterAudit, series string) consignment.SeriesAudit {
	t.Helper()
	series = consignment.NormaliseSeries(series)
	for _, s := range a.Series {
		if s.Series == series {
			return s
		}
	}
	t.Fatalf("the series %q is not in the audit", series)
	return consignment.SeriesAudit{}
}

func TestVoidNumberRefusesANumberAlreadyInUse(t *testing.T) {
	s := tempStore(t)
	note, err := s.newNote("BLR", consignment.Today())
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.voidNumber(note.Number, consignment.Cancellation{Reason: "leaf spoiled"})
	if err == nil {
		t.Fatal("a number in use was voided, which would have lost whatever it accounted for")
	}
	if !strings.Contains(err.Error(), "cancelled, not voided") {
		t.Errorf("the refusal does not say what to do instead: %v", err)
	}
}

func TestRenumberOnlyGoesForwardAndOnlyOntoAFreeNumber(t *testing.T) {
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)
	for i := 0; i < 3; i++ {
		if _, err := s.newNote("BLR", today); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name    string
		from    int
		to      int
		wantErr string
	}{
		{"onto a number already in use", 1, 2, "already in this register"},
		{"backwards into the used run", 3, 2, "already in this register"},
		{"onto its own number", 1, 1, "already carries that number"},
		{"to zero", 1, 0, "starts at 1"},
		{"from a number that is not there", 99, 10, "no note numbered"},
		{"forward past the end", 1, 4, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.renumber(consignment.Number{Series: "BLR", Seq: c.from}, c.to)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("renumber: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("renumber(%d -> %d) was allowed", c.from, c.to)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("the refusal is %q, want it to mention %q", err, c.wantErr)
			}
		})
	}
}

func TestRenumberIsHowAReuseIsPutRight(t *testing.T) {
	// Two notes on one number is the worst thing that can happen to the
	// register, and the way out is to move one of them to the end.
	s := tempStore(t)
	path := s.path
	today := consignment.MustDate(2026, time.April, 6)

	// Write a file with a doubled number, as a hand-edited one or a migration
	// off paper could.
	data := fileData{
		Books: []consignment.Book{{Series: "BLR", StartAt: 1, SeqWidth: 4}},
		Notes: []consignment.Note{
			{Number: consignment.Number{Series: "BLR", Seq: 1}, Date: today, Status: consignment.StatusBooked},
			{Number: consignment.Number{Series: "BLR", Seq: 2}, Date: today, Status: consignment.StatusBooked},
			{Number: consignment.Number{Series: "BLR", Seq: 2}, Date: today.AddDays(1), Status: consignment.StatusBooked},
		},
	}.normalise()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err = openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	before := consignment.Audit(s.snapshot().Notes, s.snapshot().Books)
	if before.Intact || before.Reused != 1 {
		t.Fatalf("the test needs a reuse: intact = %v, reused = %d", before.Intact, before.Reused)
	}

	if _, err := s.renumber(consignment.Number{Series: "BLR", Seq: 2}, 3); err != nil {
		t.Fatalf("renumber: %v", err)
	}
	after := consignment.Audit(s.snapshot().Notes, s.snapshot().Books)
	if !after.Intact {
		t.Errorf("the reuse was not put right: %s\n%v", after.Headline, after.Breaks)
	}
}

func TestClearKeepsTheOfficeDetails(t *testing.T) {
	s := tempStore(t)
	if err := s.loadSample(); err != nil {
		t.Fatal(err)
	}
	before := s.snapshot()

	if err := s.clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	after := s.snapshot()

	if len(after.Notes) != 0 {
		t.Errorf("%d notes survived the clear", len(after.Notes))
	}
	if after.Carrier.Name != before.Carrier.Name {
		t.Errorf("the carrier was cleared too: %q", after.Carrier.Name)
	}
	if len(after.Books) != len(before.Books) {
		t.Errorf("the books were cleared too: %d", len(after.Books))
	}
	if after.Rates != before.Rates {
		t.Errorf("the rates were cleared too: %+v", after.Rates)
	}
}

func TestChangeRollsBackWhenSavingFails(t *testing.T) {
	// Point the register at a path inside a file, so writing it cannot work.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := &store{path: filepath.Join(blocker, "register.json"), data: fileData{}.normalise()}
	if _, err := s.newNote("BLR", consignment.Today()); err == nil {
		t.Fatal("a note was saved into a path that cannot exist")
	}
	if got := len(s.snapshot().Notes); got != 0 {
		t.Errorf("%d notes are in memory after the save failed; the change was not rolled back", got)
	}
}

func TestNormaliseDeduplicatesBooks(t *testing.T) {
	got := fileData{
		Books: []consignment.Book{
			{Series: "BLR", Label: "first"},
			{Series: "blr", Label: "second"},
			{Series: "HUB"},
		},
	}.normalise()

	if len(got.Books) != 2 {
		t.Fatalf("got %d books, want 2: two of them are the same series", len(got.Books))
	}
	// The first declaration wins. It is the one the register was built on, and
	// quietly preferring a later duplicate could move the number a series is
	// audited from.
	if got.Books[0].Label != "first" {
		t.Errorf("the first declaration did not win: %+v", got.Books[0])
	}
}

func TestNormaliseDeclaresASeriesOnlyTheNotesKnowAbout(t *testing.T) {
	// A file that came off a paper book has notes in a series nobody declared.
	// The audit needs a book to measure it against, and that book starts at the
	// lowest number the series holds — not at one, because the paper book did
	// not start there either.
	got := fileData{
		Notes: []consignment.Note{
			{Number: consignment.Number{Series: "HUB", Seq: 104}, Date: consignment.Today()},
			{Number: consignment.Number{Series: "HUB", Seq: 102}, Date: consignment.Today()},
			{Number: consignment.Number{Series: "HUB", Seq: 0}, Date: consignment.Today()},
		},
	}.normalise()

	if len(got.Books) != 1 {
		t.Fatalf("got %d books, want one for the HUB series: %+v", len(got.Books), got.Books)
	}
	if got.Books[0].Series != "HUB" {
		t.Errorf("the declared series is %q, want HUB", got.Books[0].Series)
	}
	if got.Books[0].StartAt != 102 {
		t.Errorf("the book starts at %d, want the series' own lowest number 102", got.Books[0].StartAt)
	}

	// And the audit then finds the one real hole, at 103.
	audit := consignment.Audit(got.Notes, got.Books)
	if audit.Missing != 1 {
		t.Errorf("missing = %d, want 1 (number 103): %s", audit.Missing, audit.Headline)
	}
}

func TestNormaliseDropsTheUnlabelledSeriesOnceAnotherIsNamed(t *testing.T) {
	got := fileData{
		Books: []consignment.Book{
			{SeqWidth: consignment.SeqWidthDefault, StartAt: 1},
			{Series: "BLR", Label: "Bengaluru"},
		},
	}.normalise()

	if len(got.Books) != 1 || got.Books[0].Series != "BLR" {
		t.Errorf("books = %+v, want only the named one", got.Books)
	}

	// Unless something was issued on it, in which case it stays, because it is
	// a series with notes in it and the audit has to account for them.
	kept := fileData{
		Books: []consignment.Book{
			{SeqWidth: consignment.SeqWidthDefault, StartAt: 1},
			{Series: "BLR", Label: "Bengaluru"},
		},
		Notes: []consignment.Note{{Number: consignment.Number{Seq: 1}, Date: consignment.Today()}},
	}.normalise()

	if len(kept.Books) != 2 {
		t.Errorf("books = %+v, want both: the unlabelled series has a note on it", kept.Books)
	}
}

func TestCountIn(t *testing.T) {
	notes := []consignment.Note{
		{Number: consignment.Number{Series: "BLR", Seq: 1}},
		{Number: consignment.Number{Series: "blr", Seq: 2}},
		{Number: consignment.Number{Series: "HUB", Seq: 1}},
		{Number: consignment.Number{Series: "BLR", Seq: 0}}, // unnumbered
	}
	cases := []struct {
		series string
		want   int
	}{
		{"BLR", 2},
		{"blr", 2},
		{"HUB", 1},
		{"XXX", 0},
	}
	for _, c := range cases {
		if got := countIn(notes, c.series); got != c.want {
			t.Errorf("countIn(%q) = %d, want %d", c.series, got, c.want)
		}
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	s := tempStore(t)
	if err := s.loadSample(); err != nil {
		t.Fatal(err)
	}

	snap := s.snapshot()
	snap.Notes[0].Consignor.Name = "tampered"
	snap.Notes[0].Goods[0].Description = "tampered"
	snap.Books[0].Label = "tampered"

	again := s.snapshot()
	if again.Notes[0].Consignor.Name == "tampered" {
		t.Error("the snapshot shares the notes with the store")
	}
	if again.Notes[0].Goods[0].Description == "tampered" {
		t.Error("the snapshot shares the goods lines with the store")
	}
	if again.Books[0].Label == "tampered" {
		t.Error("the snapshot shares the books with the store")
	}
}

func TestRenumberSuggestsANumberPastTheWholeRun(t *testing.T) {
	// The number a refusal suggests has to be past the whole series, not past
	// however much of it happened to be looked at first.
	s := tempStore(t)
	today := consignment.MustDate(2026, time.April, 6)
	for i := 0; i < 5; i++ {
		if _, err := s.newNote("BLR", today); err != nil {
			t.Fatal(err)
		}
	}

	_, err := s.renumber(consignment.Number{Series: "BLR", Seq: 5}, 2)
	if err == nil {
		t.Fatal("a note was renumbered onto an occupied number")
	}
	if !strings.Contains(err.Error(), "renumber to 6 or past it") {
		t.Errorf("the refusal suggests the wrong number: %v", err)
	}
}
