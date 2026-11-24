package consignment

import "strings"

// A consignment note states the freight and the declared value in words as
// well as in figures, because the words are what settles an argument about a
// figure that was written over. The words follow the Indian scale — thousand,
// lakh, crore — which is what the figures on the note are grouped in.

var onesWords = [...]string{
	"Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine",
	"Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen",
	"Seventeen", "Eighteen", "Nineteen",
}

var tensWords = [...]string{
	"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety",
}

// RupeesInWords writes the amount in words, naming the paise only where there
// are some. Zero is spelled out rather than left blank: a note whose freight is
// nil says so.
func RupeesInWords(amount Paise) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	whole := int64(amount / PaisePerRupee)
	paise := int64(amount % PaisePerRupee)

	var b strings.Builder
	if negative {
		b.WriteString("Minus ")
	}
	b.WriteString("Rupees ")
	b.WriteString(indianWords(whole))
	if paise > 0 {
		b.WriteString(" and ")
		b.WriteString(indianWords(paise))
		b.WriteString(" Paise")
	}
	b.WriteString(" only")
	return b.String()
}

// KilogramsInWords writes a weight in words, for the goods description block
// where a figure alone is easy to alter.
func KilogramsInWords(g Grams) string {
	negative := g < 0
	if negative {
		g = -g
	}

	whole := int64(g / GramsPerKilogram)
	grams := int64(g % GramsPerKilogram)

	var b strings.Builder
	if negative {
		b.WriteString("Minus ")
	}
	b.WriteString(indianWords(whole))
	b.WriteString(" Kilogram")
	if whole != 1 {
		b.WriteString("s")
	}
	if grams > 0 {
		b.WriteString(" ")
		b.WriteString(indianWords(grams))
		b.WriteString(" Gram")
		if grams != 1 {
			b.WriteString("s")
		}
	}
	return b.String()
}

// indianWords spells a non-negative whole number on the Indian scale.
func indianWords(n int64) string { return words(n, true) }

// words spells a number, putting an "and" before the last group only where
// allowAnd lets it. The multiplier of a scale word never takes one: an amount
// with two "and"s in it reads as two amounts, and this figure goes on a
// document whose whole purpose is to be unambiguous. So a hundred and twenty
// crore reads as "One Hundred Twenty Crore" and the "and" is saved for the
// units at the end of the whole amount.
func words(n int64, allowAnd bool) string {
	if n == 0 {
		return onesWords[0]
	}

	var parts []string

	// Crore and above. Beyond a crore the scale keeps going — arab, kharab —
	// but a freight bill never does, so everything above is counted in crore.
	if crore := n / 10000000; crore > 0 {
		parts = append(parts, words(crore, false)+" Crore")
		n %= 10000000
	}
	if lakh := n / 100000; lakh > 0 {
		parts = append(parts, underHundred(lakh)+" Lakh")
		n %= 100000
	}
	if thousand := n / 1000; thousand > 0 {
		parts = append(parts, underHundred(thousand)+" Thousand")
		n %= 1000
	}
	if hundred := n / 100; hundred > 0 {
		parts = append(parts, onesWords[hundred]+" Hundred")
		n %= 100
	}
	if n > 0 {
		// "and" before the last group, but only where something came before it,
		// which is how the amount is read aloud at a transport counter.
		tail := underHundred(n)
		if len(parts) > 0 && allowAnd {
			tail = "and " + tail
		}
		parts = append(parts, tail)
	}
	return strings.Join(parts, " ")
}

// underHundred spells a number from 1 to 99.
func underHundred(n int64) string {
	if n < 20 {
		return onesWords[n]
	}
	out := tensWords[n/10]
	if n%10 > 0 {
		out += " " + onesWords[n%10]
	}
	return out
}
