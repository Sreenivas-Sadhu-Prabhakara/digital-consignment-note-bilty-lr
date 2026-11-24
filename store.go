package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/digital-consignment-note-bilty-lr/internal/consignment"
)

// fileFormatVersion is written into the data file so a later version of the
// tool can tell what it is reading.
const fileFormatVersion = 1

// maxNotes is how many consignment notes the register holds. A small transport
// office books a few hundred a month; twenty thousand is years of them, and the
// file stays a file a person can open in a text editor.
const maxNotes = 20000

// fileData is the whole contents of the data file: the office's own details,
// its number series, the figures it uses and every note it has issued.
type fileData struct {
	Version int                 `json:"version"`
	Carrier consignment.Carrier `json:"carrier"`
	Books   []consignment.Book  `json:"books"`
	Rates   consignment.Rates   `json:"rates"`
	Notes   []consignment.Note  `json:"notes"`
}

// normalise puts the file's contents in standard form.
func (d fileData) normalise() fileData {
	d.Version = fileFormatVersion
	d.Carrier = d.Carrier.Normalise()
	d.Rates = d.Rates.Normalise()

	for i := range d.Notes {
		d.Notes[i] = d.Notes[i].Normalise()
	}

	// One entry per series. Where a hand-edited file declares the same series
	// twice, the first entry wins: it is the one the register was built on, and
	// quietly preferring a later duplicate could move the number a series is
	// audited from and make a gap appear or disappear.
	books := make([]consignment.Book, 0, len(d.Books))
	seen := map[string]bool{}
	for _, b := range d.Books {
		b = b.Normalise()
		if seen[b.Series] {
			continue
		}
		seen[b.Series] = true
		books = append(books, b)
	}

	// Every series the notes are in is declared, so that the audit has a book
	// to measure each one against. A series that only the notes know about is
	// declared from its own lowest number, because an office that carried on
	// from a paper book did not start at one.
	for _, n := range d.Notes {
		series := consignment.NormaliseSeries(n.Number.Series)
		if n.Number.Seq <= 0 || seen[series] {
			continue
		}
		seen[series] = true
		books = append(books, consignment.Book{
			Series:   series,
			SeqWidth: consignment.SeqWidthDefault,
			StartAt:  lowestIn(d.Notes, series),
		}.Normalise())
	}

	// The unlabelled series a fresh register starts with is dropped once the
	// office has named one of its own and put nothing in the unlabelled one.
	if len(books) > 1 {
		kept := make([]consignment.Book, 0, len(books))
		for _, b := range books {
			if b.Series == "" && b.Label == "" && countIn(d.Notes, "") == 0 {
				continue
			}
			kept = append(kept, b)
		}
		books = kept
	}

	if len(books) == 0 {
		books = append(books, consignment.Book{SeqWidth: consignment.SeqWidthDefault, StartAt: 1}.Normalise())
	}
	d.Books = books
	return d
}

// lowestIn is the lowest number a series holds, or one where it holds none.
func lowestIn(notes []consignment.Note, series string) int {
	series = consignment.NormaliseSeries(series)
	lowest := 0
	for _, n := range notes {
		if consignment.NormaliseSeries(n.Number.Series) != series || n.Number.Seq <= 0 {
			continue
		}
		if lowest == 0 || n.Number.Seq < lowest {
			lowest = n.Number.Seq
		}
	}
	if lowest == 0 {
		return 1
	}
	return lowest
}

// store keeps the register in memory and writes the whole file out after every
// change. A register of a few thousand notes is a few megabytes, so rewriting
// the file is both simpler and safer than editing it in place.
type store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

// openStore reads the data file, or starts an empty register if there is none
// yet. It does not create the file until something is saved.
func openStore(path string) (*store, error) {
	s := &store{path: path, data: fileData{}.normalise()}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return s, nil
	}

	var loaded fileData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("reading %s: %w (move the file aside to start again)", path, err)
	}
	if loaded.Version > fileFormatVersion {
		return nil, fmt.Errorf("%s was written by a newer version of this tool (format %d, this build reads %d)",
			path, loaded.Version, fileFormatVersion)
	}
	s.data = loaded.normalise()
	return s, nil
}

// snapshot is a copy of the file's contents a reader can use without holding
// the lock.
func (s *store) snapshot() fileData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := s.data
	out.Books = copyOf(s.data.Books)
	out.Notes = make([]consignment.Note, len(s.data.Notes))
	for i, n := range s.data.Notes {
		n.Goods = copyOf(n.Goods)
		out.Notes[i] = n
	}
	return out
}

// copyOf duplicates a slice, keeping an empty one empty rather than letting it
// come out as nothing at all. A register with no notes in it has to answer with
// an empty list, not with a null the page has to guess about.
func copyOf[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

// change applies a change and saves the file. If saving fails the change is
// rolled back, so what is in memory always matches what is on disk.
func (s *store) change(apply func(d *fileData) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	before := s.data
	before.Books = copyOf(s.data.Books)
	before.Notes = copyOf(s.data.Notes)

	if err := apply(&s.data); err != nil {
		s.data = before
		return err
	}
	s.data = s.data.normalise()
	if err := s.save(); err != nil {
		s.data = before
		return err
	}
	return nil
}

// save writes the whole file atomically: a temporary file in the same
// directory, flushed to disk, then renamed over the old one. A crash therefore
// leaves either the old register or the new one, never half of either — which
// matters more here than in most tools, because half a register is a register
// with holes in its numbering.
func (s *store) save() error {
	s.data.Version = fileFormatVersion

	body, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("preparing the register: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename has succeeded

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replacing %s: %w", s.path, err)
	}
	return nil
}

// putCarrier saves the office's own details.
func (s *store) putCarrier(next consignment.Carrier) error {
	return s.change(func(d *fileData) error {
		d.Carrier = next.Normalise()
		return nil
	})
}

// putRates saves the figures the register works its tax position out on.
func (s *store) putRates(next consignment.Rates) error {
	return s.change(func(d *fileData) error {
		d.Rates = next.Normalise()
		return nil
	})
}

// putBook adds or changes a number series.
//
// The start of a book that already has notes in it cannot be moved: the start
// is what the audit measures the first gap from, and moving it would make a
// hole appear or disappear without anything else changing.
func (s *store) putBook(next consignment.Book) error {
	next = next.Normalise()
	return s.change(func(d *fileData) error {
		for i, existing := range d.Books {
			if existing.Series != next.Series {
				continue
			}
			if used := countIn(d.Notes, next.Series); used > 0 && existing.StartAt != next.StartAt {
				return fmt.Errorf("this series already has %s in it, so its first number cannot be changed from %d to %d: the audit measures the first gap from it",
					pluralise(used, "note", "notes"), existing.StartAt, next.StartAt)
			}
			d.Books[i] = next
			return nil
		}
		d.Books = append(d.Books, next)
		return nil
	})
}

// newNote allocates the next number in a series and saves an otherwise empty
// note against it.
//
// Allocation is always one past the highest number accounted for. It never
// fills a gap: slotting a note into a hole would put it out of date order as
// well as leaving the hole unexplained, which is two wrong things instead of
// one. A gap is closed by accounting for the number, with voidNumber.
func (s *store) newNote(series string, today consignment.Date) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		if len(d.Notes) >= maxNotes {
			return fmt.Errorf("this register holds %d notes, which is as many as it keeps; start a new file for the new year", maxNotes)
		}

		number := consignment.NextNumber(d.Notes, d.Books, series)
		if _, err := consignment.NewNumber(number.Series, number.Seq); err != nil {
			return err
		}

		out = consignment.Note{
			Number:  number,
			Date:    today,
			Status:  consignment.StatusBooked,
			Terms:   consignment.Terms{Basis: consignment.TermsPaid},
			Charges: consignment.Charges{Basis: consignment.BasisPerQuintal},
			Tax:     consignment.TaxStated{Position: consignment.PositionUnstated},
		}.Normalise()

		d.Notes = append(d.Notes, out)
		return nil
	})
	return out, err
}

// putNote saves a note over the one already carrying its number.
//
// The number itself cannot be changed here. A note that has to be renumbered —
// because two notes ended up on one number — goes through renumber, which
// refuses to put it anywhere a note already is.
func (s *store) putNote(next consignment.Note) (consignment.Note, error) {
	next = next.Normalise()

	err := s.change(func(d *fileData) error {
		if next.Number.Seq <= 0 {
			return errors.New("a note has to carry its number: this one has none")
		}
		for i, existing := range d.Notes {
			if existing.Number.Key() != next.Number.Key() {
				continue
			}
			// The status is moved by setStatus, which checks the move. Saving
			// the form must not move it sideways.
			next.Status = existing.Status
			d.Notes[i] = next
			return nil
		}
		return fmt.Errorf("there is no note numbered %s in this register", next.Number.Format(consignment.WidthOf(d.Books, next.Number.Series)))
	})
	return next, err
}

// setStatus moves a note along, or cancels it, checking what the new status
// needs before it saves.
func (s *store) setStatus(number consignment.Number, to consignment.Status) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		for i, existing := range d.Notes {
			if existing.Number.Key() != number.Key() {
				continue
			}
			if err := consignment.CanSet(existing, to); err != nil {
				return err
			}
			existing.Status = to
			d.Notes[i] = existing
			out = existing
			return nil
		}
		return fmt.Errorf("there is no note numbered %s in this register", number.Format(consignment.WidthOf(d.Books, number.Series)))
	})
	return out, err
}

// deliver records a delivery and marks the note delivered, which is one step
// rather than two because a delivery record on a note that is not delivered is
// a half-finished thing the register then has to complain about.
//
// A record on a note that is already delivered amends it. A clerk who typed the
// receiver's name wrong has to be able to put it right: a tool that made the
// first attempt permanent would leave the register holding something everybody
// knows is untrue, which is worse than letting it be corrected. What cannot be
// amended is the number, and nothing here touches it.
func (s *store) deliver(number consignment.Number, record consignment.Delivery) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		for i, existing := range d.Notes {
			if existing.Number.Key() != number.Key() {
				continue
			}
			existing.Delivery = record.Normalise()
			if err := consignment.CheckDelivery(existing, existing.Delivery); err != nil {
				return err
			}
			if err := consignment.Advance(existing.Status, consignment.StatusDelivered); err != nil {
				return err
			}
			existing.Status = consignment.StatusDelivered
			d.Notes[i] = existing
			out = existing
			return nil
		}
		return fmt.Errorf("there is no note numbered %s in this register", number.Format(consignment.WidthOf(d.Books, number.Series)))
	})
	return out, err
}

// cancel keeps a note's number and records why the consignment was called off.
func (s *store) cancel(number consignment.Number, record consignment.Cancellation) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		for i, existing := range d.Notes {
			if existing.Number.Key() != number.Key() {
				continue
			}
			existing.Cancellation = record.Normalise()
			if err := consignment.CanSet(existing, consignment.StatusCancelled); err != nil {
				return err
			}
			existing.Status = consignment.StatusCancelled
			d.Notes[i] = existing
			out = existing
			return nil
		}
		return fmt.Errorf("there is no note numbered %s in this register", number.Format(consignment.WidthOf(d.Books, number.Series)))
	})
	return out, err
}

// voidNumber accounts for a number that was never used. It is how a gap is
// closed: the number is occupied by an entry that says why nothing was booked
// against it, and the series goes back to being unbroken.
//
// It will only take a number that is free. A number already in the register is
// accounted for, and over-writing it would lose whatever it accounted for.
func (s *store) voidNumber(number consignment.Number, record consignment.Cancellation) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		if number.Seq <= 0 {
			return errors.New("which number is being accounted for?")
		}
		if strings.TrimSpace(record.Reason) == "" {
			return errors.New("a void number has to say why it was never used — a spoiled leaf, or a number skipped and accounted for afterwards")
		}
		if len(d.Notes) >= maxNotes {
			return fmt.Errorf("this register holds %d notes, which is as many as it keeps", maxNotes)
		}

		width := consignment.WidthOf(d.Books, number.Series)
		for _, existing := range d.Notes {
			if existing.Number.Key() == number.Key() {
				return fmt.Errorf("%s is already in this register as %s; a number that has something against it is cancelled, not voided",
					number.Format(width), strings.ToLower(existing.Status.Name()))
			}
		}

		if record.Date.IsZero() {
			record.Date = consignment.Today()
		}
		out = consignment.Note{
			Number:       number,
			Date:         record.Date,
			Status:       consignment.StatusVoid,
			Cancellation: record,
			Tax:          consignment.TaxStated{Position: consignment.PositionUnstated},
		}.Normalise()

		d.Notes = append(d.Notes, out)
		return nil
	})
	return out, err
}

// renumber moves a note to another number in the same series. It is the only
// way a number changes, and it exists for one case: two notes ended up on one
// number and one of them has to move.
//
// It refuses to move a note onto a number something is already on, and it
// refuses to move one backwards into a gap, which would put the register out of
// date order. The way out of a reuse is forwards, to the end of the series.
func (s *store) renumber(from consignment.Number, toSeq int) (consignment.Note, error) {
	var out consignment.Note

	err := s.change(func(d *fileData) error {
		width := consignment.WidthOf(d.Books, from.Series)
		to, err := consignment.NewNumber(from.Series, toSeq)
		if err != nil {
			return err
		}
		if to.Key() == from.Key() {
			return fmt.Errorf("%s already carries that number", from.Format(width))
		}

		// The highest number the series has reached, worked out in full before
		// anything is refused: the number suggested in a refusal has to be past
		// the whole run, not past however much of it happened to be looked at
		// first.
		highest := 0
		occupied := false
		found := -1
		for i, existing := range d.Notes {
			if consignment.NormaliseSeries(existing.Number.Series) == to.Series && existing.Number.Seq > highest {
				highest = existing.Number.Seq
			}
			if existing.Number.Key() == to.Key() {
				occupied = true
			}
			if existing.Number.Key() == from.Key() && found < 0 {
				found = i
			}
		}
		if found < 0 {
			return fmt.Errorf("there is no note numbered %s in this register", from.Format(width))
		}
		if occupied {
			return fmt.Errorf("%s is already in this register; renumber to %d or past it instead",
				to.Format(width), highest+1)
		}
		if toSeq <= highest {
			return fmt.Errorf("%s is inside the run this series has already used, so moving a note there would put the register out of date order; renumber to %d or past it",
				to.Format(width), highest+1)
		}

		moved := d.Notes[found]
		moved.Number = to
		d.Notes[found] = moved
		out = moved
		return nil
	})
	return out, err
}

// loadSample replaces the register with the worked one.
func (s *store) loadSample() error {
	return s.change(func(d *fileData) error {
		d.Carrier = consignment.SampleCarrier()
		d.Books = []consignment.Book{consignment.SampleBook()}
		d.Rates = consignment.SampleRates()
		d.Notes = consignment.SampleNotes()
		return nil
	})
}

// clear empties the register, keeping the office's own details, its series and
// its figures — which are the same every day and the longest part to type.
//
// It is the one thing in this tool that loses notes, so it is not reachable by
// accident: the page asks twice, and the data file is a plain file the user can
// copy first.
func (s *store) clear() error {
	return s.change(func(d *fileData) error {
		d.Notes = nil
		return nil
	})
}

// countIn is how many notes a series holds.
func countIn(notes []consignment.Note, series string) int {
	series = consignment.NormaliseSeries(series)
	n := 0
	for _, note := range notes {
		if consignment.NormaliseSeries(note.Number.Series) == series && note.Number.Seq > 0 {
			n++
		}
	}
	return n
}

func pluralise(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
