package consignment

import (
	"fmt"
	"regexp"
	"strings"
)

// The registration numbers a consignment note carries, and the shape each one
// has to be in. Two of them matter beyond tidiness. The vehicle's registration
// number is a particular of the note itself — the note says which truck took
// the goods. And the recipient's registration under the goods and services tax
// is what decides whether the tax on the carriage falls on the recipient at
// all, so a number that cannot be right is worth saying so about.

// GSTINLength is how long a registration number under the goods and services
// tax is: two digits of state code, the ten characters of the permanent account
// number, an entity digit, a fixed letter and a check character.
const GSTINLength = 15

// gstinCheckAlphabet is the base-36 alphabet the check character is computed
// over, in the order the published algorithm uses.
const gstinCheckAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	gstinShape = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z]{1}[0-9A-Z]{1}[Z]{1}[0-9A-Z]{1}$`)
	panShape   = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)
	pinShape   = regexp.MustCompile(`^[1-9][0-9]{5}$`)

	// A registration number issued by a state or union territory transport
	// authority: two letters of state, up to two digits of registering office,
	// up to three letters of series, up to four digits. KA 01 AB 1234.
	vehicleStateShape = regexp.MustCompile(`^[A-Z]{2}[0-9]{1,2}[A-Z]{0,3}[0-9]{1,4}$`)

	// A Bharat series registration, which runs the year first: 21 BH 1234 AA.
	vehicleBharatShape = regexp.MustCompile(`^[0-9]{2}BH[0-9]{4}[A-Z]{1,2}$`)
)

// NormaliseCode strips the spaces, hyphens and lower case a number is typed
// with, so that "ka-01 ab 1234" and "KA01AB1234" are the same number.
func NormaliseCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// GSTINCheckCharacter computes the fifteenth character of a registration number
// from the first fourteen. The published algorithm weights the characters
// alternately by one and two over a base-36 alphabet, sums the digits of each
// product in that base, and takes what is needed to reach the next multiple of
// thirty-six.
func GSTINCheckCharacter(first14 string) (byte, error) {
	up := NormaliseCode(first14)
	if len(up) != GSTINLength-1 {
		return 0, fmt.Errorf("the check character is computed from the first %d characters, and %d were given", GSTINLength-1, len(up))
	}

	var sum int
	for i := 0; i < len(up); i++ {
		value := strings.IndexByte(gstinCheckAlphabet, up[i])
		if value < 0 {
			return 0, fmt.Errorf("%q is not a character a registration number is made of", string(up[i]))
		}
		weight := 1
		if i%2 == 1 {
			weight = 2
		}
		product := value * weight
		sum += product/len(gstinCheckAlphabet) + product%len(gstinCheckAlphabet)
	}

	remainder := sum % len(gstinCheckAlphabet)
	index := (len(gstinCheckAlphabet) - remainder) % len(gstinCheckAlphabet)
	return gstinCheckAlphabet[index], nil
}

// CheckGSTIN reports what is wrong with a registration number, or nothing where
// it is in order. An empty number is not an error here: a consignor who is not
// registered has none, and whether that matters is a question for the tax
// position and not for this check.
func CheckGSTIN(s string) error {
	up := NormaliseCode(s)
	if up == "" {
		return nil
	}
	if len(up) != GSTINLength {
		return fmt.Errorf("a registration number is %d characters and this one is %d", GSTINLength, len(up))
	}
	if !gstinShape.MatchString(up) {
		return fmt.Errorf("%s is not shaped like a registration number (two digits of state, then a ten-character account number, then an entity digit, a Z and a check character)", up)
	}
	want, err := GSTINCheckCharacter(up[:GSTINLength-1])
	if err != nil {
		return err
	}
	if up[GSTINLength-1] != want {
		return fmt.Errorf("%s fails its own check character: the first fourteen characters give %s, not %s", up, string(want), string(up[GSTINLength-1]))
	}
	return nil
}

// GSTINState is the two-digit state code a registration number starts with.
func GSTINState(s string) string {
	up := NormaliseCode(s)
	if len(up) < 2 {
		return ""
	}
	return up[:2]
}

// PANInGSTIN is the ten-character account number inside a registration number.
func PANInGSTIN(s string) string {
	up := NormaliseCode(s)
	if len(up) < 12 {
		return ""
	}
	return up[2:12]
}

// CheckPAN reports whether a permanent account number is the right shape. There
// is no published check digit on it, so a number of the right shape may still
// not be anybody's.
func CheckPAN(s string) error {
	up := NormaliseCode(s)
	if up == "" {
		return nil
	}
	if !panShape.MatchString(up) {
		return fmt.Errorf("%s is not shaped like an account number (five letters, four digits, a letter)", up)
	}
	return nil
}

// CheckPIN reports whether a postal index number is the right shape.
func CheckPIN(s string) error {
	up := NormaliseCode(s)
	if up == "" {
		return nil
	}
	if !pinShape.MatchString(up) {
		return fmt.Errorf("%s is not a six-digit postal index number", up)
	}
	return nil
}

// CheckVehicleNumber reports whether a registration number is shaped like one a
// transport authority issues, in either the state form or the Bharat series.
// It is a shape check and nothing more: this tool is connected to no vehicle
// register, so a number of the right shape may belong to no vehicle at all.
func CheckVehicleNumber(s string) error {
	up := NormaliseCode(s)
	if up == "" {
		return nil
	}
	if len(up) < 5 {
		return fmt.Errorf("%s is too short to be a registration number", up)
	}
	if len(up) > 11 {
		return fmt.Errorf("%s is too long to be a registration number", up)
	}
	if vehicleBharatShape.MatchString(up) || vehicleStateShape.MatchString(up) {
		return nil
	}
	return fmt.Errorf("%s is not shaped like a registration number (KA 01 AB 1234, or 21 BH 1234 AA for a Bharat series vehicle)", up)
}

// PrintVehicleNumber spaces a registration number the way it is painted on the
// vehicle: KA 01 AB 1234. A number that does not parse is printed as it was
// typed, because a note has to show what the clerk wrote.
func PrintVehicleNumber(s string) string {
	up := NormaliseCode(s)
	if up == "" {
		return ""
	}

	if vehicleBharatShape.MatchString(up) {
		// 21 BH 1234 AA
		return up[:2] + " " + up[2:4] + " " + up[4:8] + " " + up[8:]
	}
	if !vehicleStateShape.MatchString(up) {
		return up
	}

	// State letters, then the digits of the registering office, then the
	// letters of the series, then the number.
	i := 2
	j := i
	for j < len(up) && up[j] >= '0' && up[j] <= '9' {
		j++
	}
	office := up[i:j]

	k := j
	for k < len(up) && up[k] >= 'A' && up[k] <= 'Z' {
		k++
	}
	series := up[j:k]
	number := up[k:]

	parts := []string{up[:2], office}
	if series != "" {
		parts = append(parts, series)
	}
	if number != "" {
		parts = append(parts, number)
	}
	return strings.Join(parts, " ")
}
