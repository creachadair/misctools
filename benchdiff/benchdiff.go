// Program benchdiff runs two Go benchmarks and combines their results to
// provide a comparison report.
//
// Usage:
//   cd path/to/git/repo
//   benchdiff -before b1 -after b2
//
// The names b1 and b2 must be branches or something else that can be the
// target of "git checkout".
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	beforeBranch = flag.String("before", "", "Before branch (defaults to master)")
	beforeTest   = flag.String("beforetest", "", "Before test (defaults to .)")
	afterBranch  = flag.String("after", "", "After branch (defaults to current)")
	afterTest    = flag.String("aftertest", "", "After test (defaults to -beforetest)")
	benchPattern = flag.String("match", ".", "Run benchmarks matching this regexp")
	washLevel    = flag.Float64("wash", 2, "Percentage below which differences are a wash")
	ignoreErr    = flag.Bool("ignore-error", false, "Ignore errors from the test runner")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	curBranch, err := gitCurrentBranch(ctx)
	if err != nil {
		log.Fatalf("Getting current branch: %v", err)
	}
	if *beforeBranch == "" {
		*beforeBranch = "master"
	}
	if *afterBranch == "" {
		*afterBranch = curBranch
	}
	if *beforeBranch == *afterBranch {
		log.Fatalf("The -before and -after branches must be different (%q)", *beforeBranch)
	}
	if *afterTest == "" {
		*afterTest = *beforeTest
	}

	// Run the "before" benchmarks.
	fmt.Fprintf(os.Stderr, "Running benchmarks in %q...\n", *beforeBranch)
	if err := gitCheckout(ctx, *beforeBranch); err != nil {
		log.Fatalf("Checking out %q: %v", *beforeBranch, err)
	}
	start := time.Now()
	before, err := runBenchmark(ctx, *beforeTest)
	if err != nil {
		log.Fatalf("Running -before benchmark: %v", err)
	}
	fmt.Fprintf(os.Stderr, "BEFORE [done] %d results, %v elapsed\n\n", len(before), time.Since(start))

	// Run the "after" benchmarks.
	fmt.Fprintf(os.Stderr, "Running benchmarks in %q...\n", *afterBranch)
	if err := gitCheckout(ctx, *afterBranch); err != nil {
		log.Fatalf("Checking out %q: %v", *afterBranch, err)
	}
	start = time.Now()
	after, err := runBenchmark(ctx, *afterTest)
	if err != nil {
		log.Fatalf("Running -after benchmark: %v", err)
	}
	fmt.Fprintf(os.Stderr, "AFTER [done] %d results, %v elapsed\n\n", len(after), time.Since(start))

	// Try to get back to where we started.
	if err := gitCheckout(ctx, curBranch); err != nil {
		log.Printf("Warning: unable to switch back to %q: %v", curBranch, err)
	}

	// Summarize the results as a table to stdout.
	hasMem := hasMemStats(before) || hasMemStats(after)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)

	fmt.Fprint(w, "BENCHMARK\tBEFORE\tAFTER\tSPEEDUP (%)")
	if hasMem {
		fmt.Fprint(w, "\tB/op (B)\tB/op (A)\tA/op (B)\tA/op (A)")
	}
	fmt.Fprintln(w)
	for _, b := range joinResults(before, after) {
		b.format(w, hasMem)
	}
	w.Flush()
}

func gitCheckout(ctx context.Context, branch string) error {
	return exec.CommandContext(ctx, "git", "checkout", branch).Run()
}

func gitCurrentBranch(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type result struct {
	Name        string
	Count       int64
	TimePerOp   time.Duration
	BytesPerOp  int64 // may not be present
	AllocsPerOp int64 // may not be present
}

func runBenchmark(ctx context.Context, test string) ([]result, error) {
	out, err := exec.CommandContext(ctx, "go", "test", "-bench="+*benchPattern, "-run=^NONE", test).Output()
	if err != nil {
		if *ignoreErr {
			log.Printf("Ignored error from test runner: %v", err)
		} else {
			return nil, err
		}
	}
	var res []result
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		fields := strings.Fields(line)
		r := result{Name: fields[0], Count: parseInt(fields[1])}
		for i := 2; i+1 < len(fields); i += 2 {
			switch fields[i+1] {
			case "ns/op":
				r.TimePerOp = time.Duration(parseInt(fields[i])) / time.Nanosecond
			case "B/op":
				r.BytesPerOp = parseInt(fields[i])
			case "allocs/op":
				r.AllocsPerOp = parseInt(fields[i])
			}
		}
		res = append(res, r)
	}
	return res, nil
}

func hasMemStats(rs []result) bool {
	for _, r := range rs {
		if r.BytesPerOp > 0 && r.AllocsPerOp > 0 {
			return true
		}
	}
	return false
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

type joined struct {
	Name                 string
	OldCount, NewCount   int64
	OldTime, NewTime     time.Duration
	OldBytes, NewBytes   int64
	OldAllocs, NewAllocs int64
}

func (b joined) format(w io.Writer, mem bool) {
	fmt.Fprintf(w, "%s\t%d\t%d\t", b.Name, b.OldTime, b.NewTime)
	if b.OldTime == 0 {
		// This benchmark did not exist in the original sample. Avert a zero
		// division.
		fmt.Fprint(w, "[new]")
	} else if b.NewTime == 0 {
		// This benchmark is missing from the comparison sample.
		fmt.Fprint(w, "[gone]")
	} else {
		sp := 100 * float64(b.OldTime-b.NewTime) / float64(b.OldTime)
		if math.Abs(sp) > *washLevel {
			fmt.Fprintf(w, "%.1f", sp)
		} else {
			fmt.Fprint(w, "~")
		}
	}
	if mem {
		fmt.Fprintf(w, "\t%d\t%d\t%d\t%d", b.OldBytes, b.NewBytes, b.OldAllocs, b.NewAllocs)
	}
	fmt.Fprintln(w)
}

func joinResults(old, new []result) []joined {
	var res []joined
	m := make(map[string]int)
	for _, b := range old {
		m[b.Name] = len(res)
		res = append(res, joined{
			Name:      b.Name,
			OldCount:  b.Count,
			OldTime:   b.TimePerOp,
			OldBytes:  b.BytesPerOp,
			OldAllocs: b.AllocsPerOp,
		})
	}
	for _, b := range new {
		if p, ok := m[b.Name]; ok {
			res[p].NewCount = b.Count
			res[p].NewTime = b.TimePerOp
			res[p].NewBytes = b.BytesPerOp
			res[p].NewAllocs = b.AllocsPerOp
		} else {
			res = append(res, joined{
				Name:      b.Name,
				NewCount:  b.Count,
				NewTime:   b.TimePerOp,
				NewBytes:  b.BytesPerOp,
				NewAllocs: b.AllocsPerOp,
			})
		}
	}
	return res
}
