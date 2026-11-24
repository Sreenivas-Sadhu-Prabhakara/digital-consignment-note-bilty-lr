package consignment

import "testing"

func TestRupeesInWords(t *testing.T) {
	cases := []struct {
		name string
		in   Paise
		want string
	}{
		{"zero", 0, "Rupees Zero only"},
		{"one paisa", 1, "Rupees Zero and One Paise only"},
		{"one rupee", 100, "Rupees One only"},
		{"one rupee fifty", 150, "Rupees One and Fifty Paise only"},
		{"nineteen", 1900, "Rupees Nineteen only"},
		{"twenty", 2000, "Rupees Twenty only"},
		{"twenty-one", 2100, "Rupees Twenty One only"},
		{"the hundreds boundary", 10000, "Rupees One Hundred only"},
		{"a hundred and one", 10100, "Rupees One Hundred and One only"},
		{"a hundred and ten", 11000, "Rupees One Hundred and Ten only"},
		{"the thousands boundary", 100000, "Rupees One Thousand only"},
		{"four figures", 1245000, "Rupees Twelve Thousand Four Hundred and Fifty only"},
		{"the lakh boundary", 10000000, "Rupees One Lakh only"},
		{"lakhs and thousands", 12345600, "Rupees One Lakh Twenty Three Thousand Four Hundred and Fifty Six only"},
		{"the crore boundary", 1000000000, "Rupees One Crore only"},
		{"crores and lakhs", 1250000000, "Rupees One Crore Twenty Five Lakh only"},
		{"over a hundred crore", 120000000000, "Rupees One Hundred Twenty Crore only"},
		{"with paise at the end", 1245075, "Rupees Twelve Thousand Four Hundred and Fifty and Seventy Five Paise only"},
		{"negative", -10000, "Minus Rupees One Hundred only"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RupeesInWords(c.in); got != c.want {
				t.Errorf("RupeesInWords(%d) =\n  %q\nwant\n  %q", c.in, got, c.want)
			}
		})
	}
}

func TestRupeesInWordsNeverSaysAndWithNothingBeforeIt(t *testing.T) {
	// "and" reads the amount as it is spoken at a counter, so it only appears
	// where a larger group came before it. A bare amount under a hundred has
	// nothing to hang it on.
	for _, amount := range []Paise{100, 500, 1900, 2100, 9900} {
		got := RupeesInWords(amount)
		if len(got) > 8 && got[7:11] == "and " {
			t.Errorf("RupeesInWords(%d) = %q starts with an \"and\"", amount, got)
		}
	}
}

func TestKilogramsInWords(t *testing.T) {
	cases := []struct {
		name string
		in   Grams
		want string
	}{
		{"zero", 0, "Zero Kilograms"},
		{"one kilogram", GramsPerKilogram, "One Kilogram"},
		{"two kilograms", 2 * GramsPerKilogram, "Two Kilograms"},
		{"one gram", 1, "Zero Kilograms One Gram"},
		{"a kilogram and a half", 1500, "One Kilogram Five Hundred Grams"},
		{"a quintal", GramsPerQuintal, "One Hundred Kilograms"},
		{"a tonne", GramsPerTonne, "One Thousand Kilograms"},
		{"eleven tonnes", 11000 * GramsPerKilogram, "Eleven Thousand Kilograms"},
		{"four thousand eight hundred", 4800 * GramsPerKilogram, "Four Thousand Eight Hundred Kilograms"},
		{"negative", -GramsPerKilogram, "Minus One Kilogram"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := KilogramsInWords(c.in); got != c.want {
				t.Errorf("KilogramsInWords(%d) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
