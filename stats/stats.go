// Binary stats computes basic statistics on a column of values read from files
// specified on the command-line, or from standard input.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"os"
	"regexp"
	"strings"
)

var (
	doCat  = flag.Bool("cat", false, "Catenate input to stdout")
	doSum  = flag.Bool("sum", false, "Print sum of entries")
	doMin  = flag.Bool("min", false, "Print minimum entry")
	doMax  = flag.Bool("max", false, "Print maximum entry")
	doMean = flag.Bool("mean", false, "Print arithmetic mean")
	doVar  = flag.Bool("var", false, "Print sample variance")
	doDev  = flag.Bool("stdev", false, "Print sample standard deviation")
	doTrim = flag.Bool("trim", false, "Trim leading and trailing whitespace")

	splitter  = flag.String("split", "", `Split input lines on this regexp ("" means don't split)`)
	field     = flag.Int("field", 0, "Field to select (1-based; use 0 for the entire line)")
	precision = flag.Int("prec", 1, "Number of digits of precision for fractional values")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: stats [options] <input-file>...

Print basic statistics on a column of values read from the given input files.
If no files are specified, input is read from stdin.  Files are read in the
order specified; use the special name "-" to read from stdin explicitly.

Options:`)
		flag.PrintDefaults()
	}
}

type stats struct {
	sum      big.Rat
	sda, sdq big.Rat
	min, max *big.Rat
	count    int64
}

// Sum returns the sum of all elements passed to s.Add so far.
func (s *stats) Sum() *big.Rat { return &s.sum }

// Min returns the minimum element seen so far, or nil.
func (s *stats) Min() *big.Rat { return s.min }

// Max returns the maximum element seen so far, or nil.
func (s *stats) Max() *big.Rat { return s.max }

// Mean returns the arithmetic mean of the statistics gathered so far.
// Returns nil if no values have been gathered.
func (s *stats) Mean() *big.Rat {
	if s.count == 0 {
		return nil
	}
	c := big.NewRat(1, s.count)
	return c.Mul(c, &s.sum)
}

// Var returns the sample variacne of the statistics gathered so far.
func (s *stats) Var() *big.Rat {
	return new(big.Rat).Mul(&s.sdq, big.NewRat(1, s.count-1))
}

// Count returns the number of elements seen so far.
func (s *stats) Count() int64 { return s.count }

// Add adds the value denoted by s to the sum and updates min/max as needed.
func (s *stats) Add(v *big.Rat) {
	s.sum.Add(&s.sum, v)
	s.count++
	if s.min == nil || v.Cmp(s.min) == -1 {
		s.min = v
	}
	if s.max == nil || v.Cmp(s.max) == 1 {
		s.max = v
	}
	sdaNext := new(big.Rat)
	tmp := new(big.Rat)
	sdaNext.Add(&s.sda, tmp.Sub(v, &s.sda).Mul(tmp, big.NewRat(1, s.count)))

	sdqNext := new(big.Rat)
	tmp2 := new(big.Rat)
	sdqNext.Add(&s.sdq, tmp.Sub(v, &s.sda).Mul(tmp, tmp2.Sub(v, sdaNext)))
	s.sda.Set(sdaNext)
	s.sdq.Set(sdqNext)
}

func newPicker(re string, n int) *picker {
	if re == "" {
		return &picker{field: n}
	}
	return &picker{regexp.MustCompile(re), n}
}

type picker struct {
	*regexp.Regexp
	field int
}

// Pick returns the value selected by the current settings from s.
func (p picker) Pick(s string) (*big.Rat, error) {
	var field string
	if p.Regexp == nil || p.field <= 0 {
		field = strings.TrimSpace(s)
	} else if fields := p.Split(s, -1); len(fields) < p.field {
		return nil, fmt.Errorf("field %d out of range (%d found)", p.field, len(fields))
	} else {
		field = fields[p.field-1]
	}

	v, ok := big.NewRat(0, 1).SetString(field)
	if ok {
		return v, nil
	}
	return nil, fmt.Errorf("invalid number format for %q", field)
}

func ratString(r *big.Rat) string {
	if r == nil {
		return "0"
	} else if r.IsInt() || *precision <= 0 {
		return r.RatString()
	}
	return r.FloatString(*precision)
}

func trim(s string) string {
	if *doTrim {
		return strings.TrimSpace(s)
	}
	return strings.TrimRight(s, "\n")
}

func main() {
	flag.Parse()

	p := newPicker(*splitter, *field)
	s := new(stats)

	var w *bufio.Writer
	if *doCat {
		w = bufio.NewWriter(os.Stdout)
	}

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"-"}
	}
	for _, path := range args {
		var r io.ReadCloser
		if path == "-" {
			path = "<stdin>"
			r = os.Stdin
		} else if f, err := os.Open(path); err == nil {
			r = f
		} else {
			r = f
		}

		br := bufio.NewReader(r)
		for ln := 1; ; ln++ {
			line, err := br.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				fail("In %s: line %d: %v", path, ln, err)
			}

			v, err := p.Pick(trim(line))
			if err != nil {
				log.Printf("In %s: line %d: %v", path, ln, err)
				continue
			}
			s.Add(v)

			if *doCat {
				if _, err := w.Write([]byte(line)); err != nil {
					fail("Output: %v", err)
				}
			}
		}
		r.Close()
	}
	if w != nil {
		if err := w.Flush(); err != nil {
			log.Printf("Flushing output failed: %v", err)
		}
	}

	out := []string{fmt.Sprintf("n=%d", s.Count())}
	if *doSum {
		out = append(out, fmt.Sprintf("sum=%v", ratString(s.Sum())))
	}
	if *doMin {
		out = append(out, fmt.Sprintf("min=%v", ratString(s.Min())))
	}
	if *doMax {
		out = append(out, fmt.Sprintf("max=%v", ratString(s.Max())))
	}
	if *doMean {
		out = append(out, fmt.Sprintf("avg=%v", ratString(s.Mean())))
	}
	if *doVar {
		out = append(out, fmt.Sprintf("var=%v", ratString(s.Var())))
	}
	if *doDev {
		d, _ := s.Var().Float64()
		out = append(out, fmt.Sprintf("sdv=%.2f", math.Sqrt(d)))
	}
	if *doCat {
		fmt.Fprintln(os.Stderr, strings.Join(out, ", "))
	} else {
		fmt.Println(strings.Join(out, ", "))
	}
}

func fail(msg string, args ...any) {
	log.Printf(msg, args...)
	os.Exit(1)
}
