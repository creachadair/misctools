package statement

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/extrame/xls"

	"local/currency"
)

// Parse extracts the gain/loss entries from the statement in data, returning
// those matched by filter (or all, if filter == nil).
func Parse(data []byte, filter func(*Entry) bool) ([]*Entry, error) {
	w, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return nil, err
	}
	return parseEntries(w.ReadAllCells(math.MaxInt32), filter)
}

// parseEntries converts a slice of rows and columns into entries.
// If filter == nil all entries are returned, otherwise only those for which
// the filter returns true.
func parseEntries(rows [][]string, filter func(*Entry) bool) ([]*Entry, error) {
	var parser func([]string) (*Entry, error)
	var i int
nextRow:
	for ; i < len(rows); i++ {
		row := rows[i]
		if len(row) != len(parse) {
			continue
		}
		for _, key := range row {
			if _, ok := parse[strings.ToLower(key)]; !ok {
				continue nextRow
			}
		}
		// Found the header row.
		parser = newParser(row)
		break
	}

	if parser == nil {
		return nil, errors.New("unable to locate the header")
	}

	var entries []*Entry
	for i++; i < len(rows); i++ {
		if len(rows[i]) == 0 {
			break // last row
		}
		e, err := parser(rows[i])
		if err != nil {
			return nil, fmt.Errorf("row %d: %v", i+1, err)
		}
		if filter == nil || filter(e) {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Acquired.Before(entries[j].Acquired)
	})
	for i, e := range entries {
		e.Index = i + 1
	}
	return entries, nil
}

// parse maps column names to functions parsing their values.
var parse = map[string]func(string, *Entry) error{
	"acquired date": func(s string, into *Entry) error {
		t, err := time.Parse("01/02/2006", s)
		into.Acquired = t
		return err
	},
	"plan name": func(s string, into *Entry) error {
		into.Plan = s
		return nil
	},
	"acquired price": func(s string, into *Entry) error {
		c, err := currency.ParseUSD(s)
		into.IssuePrice = c
		return err
	},
	"acquired via": func(s string, into *Entry) error {
		into.Via = s
		return nil
	},
	"shares available for sale": func(s string, into *Entry) error {
		n, err := strconv.Atoi(s)
		into.Available = n
		return err
	},
	"current market value": func(s string, into *Entry) error {
		c, err := currency.ParseUSD(s)
		into.Price = c
		return err
	},
	"unrealized total gain/loss": func(s string, into *Entry) error {
		c, err := currency.ParseUSD(s)
		into.Gain = c
		return err
	},
}

// An Entry represents a group of shares acquired at a particular time.
type Entry struct {
	Index      int            // batch index
	Acquired   time.Time      // when the shares were issued
	Plan       string         // under what plan
	Available  int            // how many shares are available
	Via        string         // how they were received
	IssuePrice currency.Value // price per share at issue
	Price      currency.Value // value per share currently (estimated)
	Gain       currency.Value // capital gain/loss per share (via estimated value)
}

// Format returns a description of n shares of e. If n < 0, the total available
// share count is used.
func (e *Entry) Format(n int) string {
	if n < 0 || n > e.Available {
		n = e.Available
	}
	return fmt.Sprintf("%2d %s shares acquired %s : price %s value %s gains %s",
		n, e.Plan, e.Acquired.Format("2006-01-02"),
		e.IssuePrice.USD(), e.Price.USD(), e.Gain.USD())
}

// newParser constructs a row parsing function given a header row.
func newParser(header []string) func([]string) (*Entry, error) {
	parser := make([]func(string, *Entry) error, len(header))
	for i, elt := range header {
		if f, ok := parse[strings.ToLower(elt)]; ok {
			parser[i] = f
		} else {
			return nil
		}
	}
	return func(row []string) (*Entry, error) {
		if len(row) != len(parser) {
			return nil, fmt.Errorf("invalid row: have %d columns, want %d", len(row), len(header))
		}
		var entry Entry
		for i, elt := range row {
			if err := parser[i](elt, &entry); err != nil {
				return nil, fmt.Errorf("parsing %q: %v", header[i], err)
			}
		}
		if n := currency.Value(entry.Available); n > 0 {
			entry.Price /= n
			entry.Gain /= n
		}
		return &entry, nil
	}
}
