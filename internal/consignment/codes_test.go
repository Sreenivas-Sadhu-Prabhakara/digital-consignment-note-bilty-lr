package consignment

import "testing"

func TestNormaliseCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"ka-01 ab 1234", "KA01AB1234"},
		{"KA 01 AB 1234", "KA01AB1234"},
		{"  29aaacs1234j1zt  ", "29AAACS1234J1ZT"},
		{"29-AAACS-1234-J1ZT", "29AAACS1234J1ZT"},
		{"!!!", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := NormaliseCode(c.in); got != c.want {
				t.Errorf("NormaliseCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// goodGSTINs are numbers whose fifteenth character this tool computes and finds
// to agree, built by taking a well-formed first fourteen and appending the
// check character the published algorithm gives.
func goodGSTINs(t *testing.T) []string {
	t.Helper()
	out := make([]string, 0, 6)
	for _, first14 := range []string{
		"29AAACS1234J1Z",
		"29AAFCM4567K1Z",
		"29AAGCV7788L1Z",
		"29AAHCS2211M1Z",
		"27AABCD1234E1Z",
		"07AAACP8888Q2Z",
	} {
		check, err := GSTINCheckCharacter(first14)
		if err != nil {
			t.Fatalf("GSTINCheckCharacter(%q): %v", first14, err)
		}
		out = append(out, first14+string(check))
	}
	return out
}

func TestCheckGSTINAcceptsNumbersThatAgreeWithTheirCheckCharacter(t *testing.T) {
	for _, gstin := range goodGSTINs(t) {
		if err := CheckGSTIN(gstin); err != nil {
			t.Errorf("CheckGSTIN(%q) = %v, want no error", gstin, err)
		}
	}
}

func TestCheckGSTIN(t *testing.T) {
	good := goodGSTINs(t)[0]

	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty is not an error here", "", false},
		{"a number that agrees with its check character", good, false},
		{"the same number typed with spaces", " " + good[:2] + " " + good[2:], false},
		{"the last character changed", good[:14] + wrongCheck(good[14]), true},
		{"too short", good[:14], true},
		{"too long", good + "9", true},
		{"wrong shape — digits where letters belong", "2912345S1234J1ZT", true},
		{"wrong shape — no Z in the fourteenth place", good[:13] + "A" + string(good[14]), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckGSTIN(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("CheckGSTIN(%q) = %v, want an error: %v", c.in, err, c.wantErr)
			}
		})
	}
}

// wrongCheck returns a check character that is not the given one.
func wrongCheck(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}

func TestGSTINCheckCharacterIsStableAndRefusesBadInput(t *testing.T) {
	first, err := GSTINCheckCharacter("29AAACS1234J1Z")
	if err != nil {
		t.Fatalf("GSTINCheckCharacter: %v", err)
	}
	again, err := GSTINCheckCharacter("29aaacs1234j1z")
	if err != nil {
		t.Fatalf("GSTINCheckCharacter on lower case: %v", err)
	}
	if first != again {
		t.Errorf("case changed the check character: %q then %q", string(first), string(again))
	}

	for _, bad := range []string{"", "29AAACS1234J1", "29AAACS1234J1ZT9"} {
		if _, err := GSTINCheckCharacter(bad); err == nil {
			t.Errorf("GSTINCheckCharacter(%q) gave no error", bad)
		}
	}
}

func TestGSTINPartsCameOut(t *testing.T) {
	gstin := goodGSTINs(t)[0]
	if got, want := GSTINState(gstin), "29"; got != want {
		t.Errorf("GSTINState = %q, want %q", got, want)
	}
	if got, want := PANInGSTIN(gstin), "AAACS1234J"; got != want {
		t.Errorf("PANInGSTIN = %q, want %q", got, want)
	}
	if got := GSTINState("2"); got != "" {
		t.Errorf("GSTINState on a short number = %q, want empty", got)
	}
	if got := PANInGSTIN("29AAA"); got != "" {
		t.Errorf("PANInGSTIN on a short number = %q, want empty", got)
	}
}

func TestCheckPAN(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"AABCD1234E", false},
		{"aabcd1234e", false},
		{"AABCD1234", true},
		{"AABC1234E", true},
		{"AABCD12345", true},
		{"1ABCD1234E", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if err := CheckPAN(c.in); (err != nil) != c.wantErr {
				t.Errorf("CheckPAN(%q) = %v, want an error: %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestCheckPIN(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"560022", false},
		{"580030", false},
		{"56002", true},
		{"5600220", true},
		{"060022", true}, // a postal index number does not start with a zero
		{"56002A", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if err := CheckPIN(c.in); (err != nil) != c.wantErr {
				t.Errorf("CheckPIN(%q) = %v, want an error: %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestCheckVehicleNumber(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty is not an error here", "", false},
		{"an ordinary state registration", "KA01AB1234", false},
		{"typed with spaces and hyphens", "ka-01 ab 1234", false},
		{"a one-digit registering office", "KA1AB1234", false},
		{"a single-letter series", "KA25C4471", false},
		{"a three-letter series", "KA01ABC1234", false},
		{"fewer than four digits", "KA01AB123", false},
		{"no series letters at all", "DL1C1234", false},
		{"a Bharat series", "21BH1234AA", false},
		{"a Bharat series with one letter", "21BH1234A", false},
		{"too short", "KA01", true},
		{"too long", "KA01ABCD12345", true},
		{"letters where the state code belongs", "1A01AB1234", true},
		{"nothing but letters", "ABCDEFG", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := CheckVehicleNumber(c.in); (err != nil) != c.wantErr {
				t.Errorf("CheckVehicleNumber(%q) = %v, want an error: %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestPrintVehicleNumber(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"an ordinary registration", "KA01AB1234", "KA 01 AB 1234"},
		{"typed with hyphens", "ka-01-ab-1234", "KA 01 AB 1234"},
		{"a single-letter series", "KA25C4471", "KA 25 C 4471"},
		{"a three-letter series", "KA01ABC1234", "KA 01 ABC 1234"},
		{"no series letters", "DL1C1234", "DL 1 C 1234"},
		{"a Bharat series", "21BH1234AA", "21 BH 1234 AA"},
		{"one that does not parse comes out as typed", "ABCDEFG", "ABCDEFG"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PrintVehicleNumber(c.in); got != c.want {
				t.Errorf("PrintVehicleNumber(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
