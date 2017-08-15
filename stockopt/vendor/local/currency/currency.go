// Package currency provides a fixed-precision representation of an amount of
// currency as an integer count of millicents.
package currency

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The units of currency.
const (
	Millicents = 1
	Cents      = 1000 * Millicents
	Dollars    = 100 * Cents
)

// Value represents a currency value
type Value int64

// String renders c as a value in millicents.
func (c Value) String() string { return fmt.Sprintf("%dm¢", int64(c)) }

// USD renders c as a value in U.S. dollars ($ddd.cc).
func (c Value) USD() string {
	neg := c < 0
	if neg {
		c = -c
	}
	usd := c / Dollars
	usc := (c % Dollars) / Cents
	if neg {
		return fmt.Sprintf("-$%d.%02d", usd, usc)
	}
	return fmt.Sprintf("$%d.%02d", usd, usc)
}

// The expression matching a value in USD.
var usd = regexp.MustCompile(`^-?\$?(\d+)(?:\.(\d+))?$`)

// ParseUSD parses a string denoting a value in US dollars to a Value.
func ParseUSD(s string) (Value, error) {
	m := usd.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid USD amount %q", s)
	}
	d, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid dollar amount: %v", err)
	}
	d *= Dollars

	var c, mul int64 = 0, Dollars
	for _, ch := range m[2] {
		c = (c * 10) + int64(ch-'0')
		mul /= 10
	}
	d += c * mul
	if strings.HasPrefix(m[0], "-") {
		d = -d
	}
	return Value(d), nil
}
