package pricing

import "testing"

func TestApply(t *testing.T) {
	cases := []struct {
		name     string
		subtotal int
		code     string
		want     int
	}{
		{"no code", 2000, "", 2000},
		{"ten percent", 2000, "SAVE10", 1800},
		{"code is normalized", 2000, " save25 ", 1500},
		{"bulk at threshold", 10000, "", 9500},
		{"bulk then percent", 20000, "SAVE10", 17550},
		{"unknown code", 3000, "NOPE", 3000},
	}
	for _, c := range cases {
		if got := Apply(c.subtotal, Normalize(c.code)); got != c.want {
			t.Errorf("%s: Apply(%d, %q) = %d, want %d", c.name, c.subtotal, c.code, got, c.want)
		}
	}
}
