package consignment

import "testing"

func TestParseGrams(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Grams
		wantErr bool
	}{
		{"empty is nothing", "", 0, false},
		{"whole kilograms", "1250", 1250000, false},
		{"half a kilogram", "1250.5", 1250500, false},
		{"to the gram", "1250.250", 1250250, false},
		{"one gram", "0.001", 1, false},
		{"grouped", "11,000", 11000000, false},
		{"with the unit", "1250 kg", 1250000, false},
		{"zero", "0", 0, false},
		{"negative", "-5", -5000, false},
		{"finer than a gram is refused", "1.0005", 0, true},
		{"not a weight", "heavy", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseGrams(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseGrams(%q) = %d, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGrams(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseGrams(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestGramsPrinting(t *testing.T) {
	cases := []struct {
		in       Grams
		kg       string
		quintals string
		short    string
	}{
		{0, "0.000", "0.000", "0 kg"},
		{1, "0.001", "0.000", "0.001 kg"},
		{GramsPerKilogram, "1.000", "0.010", "1 kg"},
		{1500, "1.500", "0.015", "1.5 kg"},
		{GramsPerQuintal, "100.000", "1.000", "100 kg"},
		{GramsPerTonne, "1000.000", "10.000", "1000 kg"},
		{5160 * GramsPerKilogram, "5160.000", "51.600", "5160 kg"},
	}
	for _, c := range cases {
		t.Run(c.short, func(t *testing.T) {
			if got := c.in.Kilograms(); got != c.kg {
				t.Errorf("Kilograms() = %q, want %q", got, c.kg)
			}
			if got := c.in.Quintals(); got != c.quintals {
				t.Errorf("Quintals() = %q, want %q", got, c.quintals)
			}
			if got := c.in.String(); got != c.short {
				t.Errorf("String() = %q, want %q", got, c.short)
			}
		})
	}
}

func TestChargeableWeight(t *testing.T) {
	cases := []struct {
		name            string
		actual, minimum Grams
		want            Grams
	}{
		{"no minimum", 5000 * GramsPerKilogram, 0, 5000 * GramsPerKilogram},
		{"minimum below the load", 5000 * GramsPerKilogram, 2000 * GramsPerKilogram, 5000 * GramsPerKilogram},
		{"minimum above the load", 1640 * GramsPerKilogram, 2000 * GramsPerKilogram, 2000 * GramsPerKilogram},
		{"minimum exactly the load", 2000 * GramsPerKilogram, 2000 * GramsPerKilogram, 2000 * GramsPerKilogram},
		{"nothing at all", 0, 0, 0},
		{"a minimum with no load", 0, 2000 * GramsPerKilogram, 2000 * GramsPerKilogram},
		{"a negative load is nothing", -5, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ChargeableWeight(c.actual, c.minimum); got != c.want {
				t.Errorf("ChargeableWeight(%d, %d) = %d, want %d", c.actual, c.minimum, got, c.want)
			}
		})
	}
}

func TestFreightUnits(t *testing.T) {
	cases := []struct {
		name       string
		basis      Basis
		chargeable Grams
		packages   int
		want       int64 // thousandths of a unit
	}{
		{"a quintal", BasisPerQuintal, GramsPerQuintal, 0, 1000},
		{"five tonnes in quintals", BasisPerQuintal, 5000 * GramsPerKilogram, 0, 50000},
		{"a part quintal", BasisPerQuintal, 5160 * GramsPerKilogram, 0, 51600},
		{"a tonne", BasisPerTonne, GramsPerTonne, 0, 1000},
		{"eleven tonnes", BasisPerTonne, 11000 * GramsPerKilogram, 0, 11000},
		{"a kilogram", BasisPerKilogram, GramsPerKilogram, 0, 1000},
		{"per package", BasisPerPackage, 0, 40, 40000},
		{"per package with no packages", BasisPerPackage, 5000 * GramsPerKilogram, 0, 0},
		{"a lump sum ignores the weight", BasisLumpsum, 5000 * GramsPerKilogram, 40, 1000},
		{"a lump sum with nothing on it", BasisLumpsum, 0, 0, 1000},
		{"nothing at all on a weight basis", BasisPerQuintal, 0, 0, 0},
		{"an unknown basis", Basis("by the mile"), GramsPerTonne, 5, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FreightUnits(c.basis, c.chargeable, c.packages); got != c.want {
				t.Errorf("FreightUnits(%s, %d, %d) = %d, want %d", c.basis, c.chargeable, c.packages, got, c.want)
			}
		})
	}
}

func TestFreight(t *testing.T) {
	cases := []struct {
		name       string
		rate       Paise
		basis      Basis
		chargeable Grams
		packages   int
		want       Paise
	}{
		{
			name: "₹185 a quintal on 5,160 kg",
			rate: 185 * PaisePerRupee, basis: BasisPerQuintal,
			chargeable: 5160 * GramsPerKilogram,
			want:       954600, // 51.6 quintals × ₹185 = ₹9,546.00
		},
		{
			name: "a rate with paise in it",
			rate: 21050, basis: BasisPerQuintal, // ₹210.50
			chargeable: 11000 * GramsPerKilogram,
			want:       2315500, // 110 quintals × ₹210.50 = ₹23,155.00
		},
		{
			name: "per tonne",
			rate: 2100 * PaisePerRupee, basis: BasisPerTonne,
			chargeable: 11000 * GramsPerKilogram,
			want:       2310000, // 11 tonnes × ₹2,100
		},
		{
			name: "per package",
			rate: 45 * PaisePerRupee, basis: BasisPerPackage, packages: 40,
			want: 180000, // 40 × ₹45
		},
		{
			name: "a lump sum",
			rate: 18000 * PaisePerRupee, basis: BasisLumpsum,
			chargeable: 5160 * GramsPerKilogram, packages: 40,
			want: 1800000,
		},
		{
			name: "no rate is no freight",
			rate: 0, basis: BasisPerQuintal, chargeable: 5160 * GramsPerKilogram,
			want: 0,
		},
		{
			name: "no weight on a weight basis is no freight",
			rate: 185 * PaisePerRupee, basis: BasisPerQuintal, chargeable: 0,
			want: 0,
		},
		{
			name: "a part quintal that needs rounding",
			rate: 185 * PaisePerRupee, basis: BasisPerQuintal,
			chargeable: 5163 * GramsPerKilogram,
			want:       955155, // 51.63 × ₹185 = ₹9,551.55
		},
		{
			name: "rounds to the paisa, half away from zero",
			rate: 100, basis: BasisPerKilogram, // ₹1.00 per kg
			chargeable: 1500, // 1.5 kg
			want:       150,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Freight(c.rate, c.basis, c.chargeable, c.packages)
			if got != c.want {
				t.Errorf("Freight(%d, %s, %d, %d) = %d (₹%s), want %d (₹%s)",
					c.rate, c.basis, c.chargeable, c.packages, got, got.Format(), c.want, c.want.Format())
			}
		})
	}
}

func TestUnitsPrinted(t *testing.T) {
	cases := []struct {
		name       string
		basis      Basis
		chargeable Grams
		packages   int
		want       string
	}{
		{"quintals", BasisPerQuintal, 5160 * GramsPerKilogram, 0, "51.600 qtl"},
		{"tonnes", BasisPerTonne, 11000 * GramsPerKilogram, 0, "11.000 tonne"},
		{"kilograms", BasisPerKilogram, 1500, 0, "1.500 kg"},
		{"packages", BasisPerPackage, 0, 40, "40 pkg"},
		{"a lump sum", BasisLumpsum, 5160 * GramsPerKilogram, 40, "1 trip"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := UnitsPrinted(c.basis, c.chargeable, c.packages); got != c.want {
				t.Errorf("UnitsPrinted = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBasisValidAndNamed(t *testing.T) {
	for _, b := range Bases() {
		if !b.Valid() {
			t.Errorf("%s is in Bases() but not Valid()", b)
		}
		if b.Name() == string(b) {
			t.Errorf("%s has no name of its own", b)
		}
		if b.Unit() == "" {
			t.Errorf("%s has no unit", b)
		}
	}
	if Basis("by the mile").Valid() {
		t.Error("an unknown basis reported itself valid")
	}
}
