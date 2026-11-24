package consignment

import (
	"fmt"
	"strconv"
	"strings"
)

// Grams is weight in whole grams. A bilty is written in quintals and kilograms
// and the freight is charged on a weight, so the weight has to add up exactly
// across the goods lines. Grams is small enough that a half-kilogram tare is
// a whole number and large enough that a full truckload fits with room to
// spare.
type Grams int64

const (
	// GramsPerKilogram, GramsPerQuintal and GramsPerTonne are units, not
	// statutory figures. A quintal is a hundred kilograms and is the unit road
	// freight in India is quoted in; a tonne is a thousand.
	GramsPerKilogram Grams = 1000
	GramsPerQuintal  Grams = 100 * GramsPerKilogram
	GramsPerTonne    Grams = 1000 * GramsPerKilogram
)

// Kilograms is the weight as kilograms, to three places.
func (g Grams) Kilograms() string { return scaled(int64(g), 1000, 3) }

// Quintals is the weight as quintals, to three places — the figure a freight
// rate per quintal is multiplied by.
func (g Grams) Quintals() string { return scaled(int64(g), 100000, 3) }

// String writes the weight the way a consignment note prints it: kilograms,
// trailing zeroes trimmed, with the unit.
func (g Grams) String() string {
	return strings.TrimSuffix(strings.TrimRight(g.Kilograms(), "0"), ".") + " kg"
}

// ParseGrams reads a weight written in kilograms — "1250", "1250.5", "1,250.250"
// — into whole grams. Finer than a gram is a typing mistake.
func ParseGrams(s string) (Grams, error) {
	t := strings.NewReplacer(",", "", " ", "", "kg", "", "KG", "", "Kg", "").Replace(strings.TrimSpace(s))
	if t == "" {
		return 0, nil
	}

	neg := false
	switch t[0] {
	case '-':
		neg, t = true, t[1:]
	case '+':
		t = t[1:]
	}
	if t == "" {
		return 0, fmt.Errorf("%q is not a weight", s)
	}

	whole, frac, hasFrac := strings.Cut(t, ".")
	if whole == "" {
		whole = "0"
	}
	if hasFrac && len(frac) > 3 {
		return 0, fmt.Errorf("%q is finer than a gram", s)
	}
	for frac != "" && len(frac) < 3 {
		frac += "0"
	}
	if frac == "" {
		frac = "000"
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a weight", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a weight", s)
	}

	out := Grams(w)*GramsPerKilogram + Grams(f)
	if neg {
		out = -out
	}
	return out, nil
}

// scaled writes v divided by scale as a decimal with a fixed number of places,
// rounded half away from zero. The scale and the number of places are
// independent — grams to quintals divides by a hundred thousand and prints
// three places — so the division happens first and the remainder is taken
// against the places, not against the scale.
func scaled(v, scale int64, places int) string {
	pow := int64(1)
	for i := 0; i < places; i++ {
		pow *= 10
	}

	n := divRound(v*pow, scale)
	neg := n < 0
	if neg {
		n = -n
	}
	whole, part := n/pow, n%pow

	out := strconv.FormatInt(whole, 10)
	if places > 0 {
		out += "." + leftPad(strconv.FormatInt(part, 10), places)
	}
	if neg {
		out = "-" + out
	}
	return out
}

func leftPad(s string, width int) string {
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// Basis is what the freight on a consignment is charged on. A road carrier
// quotes per quintal, per tonne, per package or as a lump sum for the trip, and
// which one it is decides what the rate gets multiplied by.
type Basis string

const (
	// BasisPerQuintal charges the chargeable weight in quintals.
	BasisPerQuintal Basis = "per-quintal"
	// BasisPerTonne charges the chargeable weight in tonnes.
	BasisPerTonne Basis = "per-tonne"
	// BasisPerKilogram charges the chargeable weight in kilograms.
	BasisPerKilogram Basis = "per-kilogram"
	// BasisPerPackage charges the number of packages.
	BasisPerPackage Basis = "per-package"
	// BasisLumpsum charges one figure for the whole consignment, which is how
	// a full truckload is booked.
	BasisLumpsum Basis = "lumpsum"
)

// Bases is every basis, in the order a booking clerk meets them.
func Bases() []Basis {
	return []Basis{BasisPerQuintal, BasisPerTonne, BasisPerKilogram, BasisPerPackage, BasisLumpsum}
}

// Valid reports whether the basis is one this tool knows.
func (b Basis) Valid() bool {
	for _, known := range Bases() {
		if b == known {
			return true
		}
	}
	return false
}

// Name is the basis in words, for a label on screen and on the note.
func (b Basis) Name() string {
	switch b {
	case BasisPerQuintal:
		return "per quintal (100 kg)"
	case BasisPerTonne:
		return "per tonne (1,000 kg)"
	case BasisPerKilogram:
		return "per kilogram"
	case BasisPerPackage:
		return "per package"
	case BasisLumpsum:
		return "lump sum for the consignment"
	default:
		return string(b)
	}
}

// Unit is the short unit the rate is quoted in, printed beside the rate.
func (b Basis) Unit() string {
	switch b {
	case BasisPerQuintal:
		return "qtl"
	case BasisPerTonne:
		return "tonne"
	case BasisPerKilogram:
		return "kg"
	case BasisPerPackage:
		return "pkg"
	case BasisLumpsum:
		return "trip"
	default:
		return ""
	}
}

// ChargeableWeight is the weight the freight is charged on: the greater of what
// the consignment actually weighs and the minimum weight the carrier books the
// trip at. A carrier who will not move a vehicle for less than fifty quintals
// charges fifty whatever the load is, and the note has to show both figures so
// the consignor can see which one was used.
func ChargeableWeight(actual, minimumCharged Grams) Grams {
	if minimumCharged > actual {
		return minimumCharged
	}
	if actual < 0 {
		return 0
	}
	return actual
}

// FreightUnits is how many of the rate's own units a consignment comes to,
// in thousandths of a unit so a part quintal is not lost. A lump sum is one
// unit whatever the load.
func FreightUnits(b Basis, chargeable Grams, packages int) int64 {
	switch b {
	case BasisPerQuintal:
		return divRound(int64(chargeable)*1000, int64(GramsPerQuintal))
	case BasisPerTonne:
		return divRound(int64(chargeable)*1000, int64(GramsPerTonne))
	case BasisPerKilogram:
		return divRound(int64(chargeable)*1000, int64(GramsPerKilogram))
	case BasisPerPackage:
		if packages < 0 {
			return 0
		}
		return int64(packages) * 1000
	case BasisLumpsum:
		return 1000
	default:
		return 0
	}
}

// Freight is what the carriage comes to: the rate times the units, rounded once
// to whole paise, half away from zero. The units are in thousandths, so the
// division happens here and not before.
func Freight(rate Paise, b Basis, chargeable Grams, packages int) Paise {
	if rate == 0 {
		return 0
	}
	return Paise(divRound(int64(rate)*FreightUnits(b, chargeable, packages), 1000))
}

// UnitsPrinted writes the units the freight was charged on, for the line on the
// note that shows the working: "12.500 qtl", "3 pkg", "1 trip".
func UnitsPrinted(b Basis, chargeable Grams, packages int) string {
	units := FreightUnits(b, chargeable, packages)
	switch b {
	case BasisPerPackage:
		return strconv.FormatInt(units/1000, 10) + " " + b.Unit()
	case BasisLumpsum:
		return "1 " + b.Unit()
	default:
		return scaled(units, 1000, 3) + " " + b.Unit()
	}
}
