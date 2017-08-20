// Package solver implements a sale optimizer for stock shares.
package solver

import (
	"sort"

	"bitbucket.org/creachadair/misctools/stockopt/currency"
)

// An Entry represents a number of shares having a common price (value) and
// capital gain per share.
type Entry struct {
	ID    interface{}    // opaque identifier
	N     int            // number of shares
	Value currency.Value // value per share
	Gain  currency.Value // gain per share
}

func (e Entry) take(n int) Entry {
	e.N = n
	return e
}

type cell struct {
	TotalValue currency.Value
	TotalGain  currency.Value
	Next       int
}

// A Solver represents the optimizer state for a collection of entries.
type Solver struct {
	entries []Entry
	table   [][]cell // one column per entry
}

// New contructs a solver from a collection of entries.
func New(es []Entry) *Solver { return &Solver{entries: es} }

// Solve returns an optimal sale plan for which the total capital gains do not
// exceed cap.
func (s *Solver) Solve(cap currency.Value) []Entry {
	var soln []Entry

	ns := s.init(cap)
	for i, col := range s.table {
		if ns > 0 {
			soln = append(soln, s.entries[i].take(ns))
		}
		ns = col[ns].Next
	}
	if ns != 0 {
		panic("nonzero offset at end")
	}
	return soln
}

func (s *Solver) init(cap currency.Value) int {
	// Lazily initialize the solution table.
	if s.table == nil {
		// Consider entries in nondecreasing order of capital gains, so that
		// local optima are monotonic.
		sort.Slice(s.entries, func(i, j int) bool {
			return s.entries[i].Gain > s.entries[j].Gain
		})

		// s.table has one column per entry, plus a sentinel to simplify setup.
		s.table = make([][]cell, len(s.entries)+1)

		// Each column has one entry per possible number of shares assigned.
		for i, e := range s.entries {
			s.table[i] = make([]cell, e.N+1)
		}
		s.table[len(s.entries)] = make([]cell, 1) // sentinel
	}

	for i := len(s.entries) - 1; i >= 0; i-- {
		col := s.table[i] // this entry's column in the solution table

		for j := range col {
			// Value and gain of j shares of this entry.
			v := s.entries[i].Value * currency.Value(j)
			g := s.entries[i].Gain * currency.Value(j)

			// Find the largest value we can combine with this assignment in
			// the next column without blowing the cap. Update this entry's
			// column to reflect that local optimum.
			next := s.table[i+1]
			for k, elt := range next {
				tv := v + elt.TotalValue
				tg := g + elt.TotalGain
				if tg <= cap && tv > col[j].TotalValue {
					col[j].TotalValue = tv
					col[j].TotalGain = tg
					col[j].Next = k
				}
			}
		}
	}

	// At this point for entry i, table[i][n] reflects the best available
	// assignment within cap, including n shares of entry[i]. Return the
	// position i of the best starting entry.
	seed := s.table[0]
	best := 0
	for i, c := range seed {
		if c.TotalValue > seed[best].TotalValue {
			best = i
		}
	}
	return best
}
