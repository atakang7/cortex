package pricing

import "strings"

// Code is a discount code as typed by a customer.
type Code string

// percentOff maps codes to a percentage taken off the subtotal.
var percentOff = map[Code]int{
	"SAVE10": 10,
	"SAVE25": 25,
}

// BulkThreshold is the subtotal, in cents, from which the bulk discount applies.
const BulkThreshold = 10000

// BulkCents is taken off any subtotal of at least BulkThreshold.
const BulkCents = 500

// Normalize trims and upper-cases a code.
func Normalize(c string) Code {
	return Code(strings.ToUpper(strings.TrimSpace(c)))
}

// Apply returns the price in cents after the bulk discount and the code.
func Apply(subtotal int, code Code) int {
	price := subtotal
	if subtotal > BulkThreshold {
		price -= BulkCents
	}
	if pct, ok := percentOff[code]; ok {
		price -= price * pct
	}
	if price < 0 {
		return 0
	}
	return price
}
