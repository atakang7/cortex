package cart

import "testing"

func TestTotal(t *testing.T) {
	c := Cart{Items: []Item{{"pen", 250, 4}, {"pad", 1000, 9}}, Code: "save10"}
	if got := c.Subtotal(); got != 10000 {
		t.Fatalf("Subtotal = %d, want 10000", got)
	}
	if got := c.Total(); got != 8550 {
		t.Fatalf("Total = %d, want 8550", got)
	}
}
