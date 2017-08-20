// Package statement parses stock gain/loss statements as issued by MSSB.
package statement

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/creachadair/misctools/stockopt/currency"
	"github.com/extrame/xls"
)

// ParseXLS extracts the gain/loss entries from the statement in data,
// returning those matched by filter (or all, if filter == nil).
func ParseXLS(data []byte, filter func(*Entry) bool) ([]*Entry, error) {
	w, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return nil, err
	}
	return parseEntries(w.ReadAllCells(math.MaxInt32), filter)
}

// ParseCSV extracts the gain/loss entries from the statement in data,
// returning those matched by filter (or all, if filter == nil).
//
// The input data are expected to have a header row containing the standard
// fields of the MSSB gain/loss report:
//
//  Acquired Date:              date as mm/dd/yyyy
//  Plan Name:                  string
//  Acquired Price:             price as $ddd.cc
//  Acquired Via:               string
//  Shares Available for Sale:  integer
//  Current Market Value:       price as $ddd.cc
//  Unrealized Total Gain/Loss: price as $ddd.cc (possibly negative)
//
func ParseCSV(data []byte, filter func(*Entry) bool) ([]*Entry, error) {
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return nil, err
	}
	return parseEntries(rows, filter)
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

const (
	acquiredDate    = "acquired date"
	acquiredPrice   = "acquired price"
	acquiredVia     = "acquired via"
	currentValue    = "current market value"
	planName        = "plan name"
	sharesAvailable = "shares available for sale"
	totalGainLoss   = "unrealized total gain/loss"
)

// fieldPos maps column names to field positions.
var fieldPos = map[string]int{
	acquiredDate:    0,
	acquiredPrice:   2,
	acquiredVia:     3,
	currentValue:    5,
	planName:        1,
	sharesAvailable: 4,
	totalGainLoss:   6,
}

// parse maps column names to functions parsing their values.
var parse = map[string]func(string, *Entry) error{
	acquiredDate: func(s string, into *Entry) error {
		t, err := time.Parse("01/02/2006", s)
		into.Acquired = t
		return err
	},
	planName: func(s string, into *Entry) error {
		into.Plan = s
		return nil
	},
	acquiredPrice: func(s string, into *Entry) error {
		c, err := currency.ParseUSD(s)
		into.IssuePrice = c
		return err
	},
	acquiredVia: func(s string, into *Entry) error {
		into.Via = s
		return nil
	},
	sharesAvailable: func(s string, into *Entry) error {
		n, err := strconv.Atoi(s)
		into.Available = n
		return err
	},
	currentValue: func(s string, into *Entry) error {
		c, err := currency.ParseUSD(s)
		into.Price = c
		return err
	},
	totalGainLoss: func(s string, into *Entry) error {
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
	return fmt.Sprintf("%2d %s shares acquired %s : issue %s price %s gains %s",
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

		// Issue price appears to be per unit; others are total.
		// Except for Historical GCUs it looks to be otherwise.
		if n := currency.Value(entry.Available); n > 0 {
			entry.Price /= n
			entry.Gain /= n
		}
		return &entry, nil
	}
}

// EntryLess reports whether a should be ordered prior to b, based on time of
// acquisition with ties split by index.
func EntryLess(a, b *Entry) bool {
	if a.Acquired.Equal(b.Acquired) {
		return a.Index < b.Index
	}
	return a.Acquired.Before(b.Acquired)
}

// WriteCSV renders entries as CSV to w.
func WriteCSV(entries []*Entry, w io.Writer) error {
	cw := csv.NewWriter(w)
	row := make([]string, len(fieldPos))
	for name, pos := range fieldPos {
		row[pos] = name
	}
	if err := cw.Write(row); err != nil {
		return err
	}
	for _, e := range entries {
		n := currency.Value(e.Available)

		row[fieldPos[acquiredDate]] = e.Acquired.Format("01/02/2006")
		row[fieldPos[planName]] = e.Plan
		row[fieldPos[sharesAvailable]] = strconv.Itoa(e.Available)
		row[fieldPos[acquiredVia]] = e.Via
		row[fieldPos[acquiredPrice]] = e.IssuePrice.USD()
		row[fieldPos[currentValue]] = (n * e.Price).USD()
		row[fieldPos[totalGainLoss]] = (n * e.Gain).USD()

		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
