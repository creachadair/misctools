// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package scanner

import (
	"fmt"
	"strconv"
)

// A Span describes a contiguous span of a source input.
type Span struct {
	Pos int // the start offset, 0-based
	End int // the end offset, 0-based (noninclusive)
}

func (s Span) String() string {
	if s.End <= s.Pos {
		return strconv.Itoa(s.Pos)
	}
	return fmt.Sprintf("%d..%d", s.Pos, s.End)
}

// A LineCol describes the line number and column offset of a location in
// source text.
type LineCol struct {
	Line   int // line number, 1-based
	Column int // byte offset of column in line, 0-based
}

func (lc LineCol) String() string { return fmt.Sprintf("%d:%d", lc.Line, lc.Column) }

// A Location describes the complete location of a range of source text,
// including line and column offsets.
type Location struct {
	Span
	First, Last LineCol
}

func (loc Location) String() string {
	if loc.First.Line == loc.Last.Line {
		return fmt.Sprintf("%d:%d-%d", loc.First.Line, loc.First.Column, loc.Last.Column)
	}
	return loc.First.String() + "-" + loc.Last.String()
}

// IsValid reports whether loc is a "valid" location, meaning it is not the
// zero location.
func (loc Location) IsValid() bool { return loc != Location{} }
