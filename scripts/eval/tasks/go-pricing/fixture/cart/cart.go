package cart

import "example.com/shop/pricing"

// Item is one line of a cart: a unit price in cents and a quantity.
type Item struct {
	Name     string
	Cents    int
	Quantity int
}

// Cart is a list of items and an optional discount code.
type Cart struct {
	Items []Item
	Code  string
}

// Subtotal is the sum of every line before discounts.
func (c Cart) Subtotal() int {
	sum := 0
	for _, it := range c.Items {
		sum += it.Cents * it.Quantity
	}
	return sum
}

// Total is the price to pay, in cents.
func (c Cart) Total() int {
	return pricing.Apply(c.Subtotal(), pricing.Normalize(c.Code))
}
