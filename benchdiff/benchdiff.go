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
	"log"
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
	afterTest    = flag.String("aftertest", "", "After test (defaults to .)")
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
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
	fmt.Fprintln(w, "BENCHMARK\tBEFORE\tAFTER\tSPEEDUP (%)")
	for _, b := range joinResults(before, after) {
		if b.OldTime == 0 {
			// If this benchmark did not exist in the original sample, we can't
			// compare.  Avert a zero division but still print out the results.
			fmt.Fprintf(w, "%s\t%d\t%d\t?\n", b.Name, b.OldTime, b.NewTime)
		} else {
			sp := float64(b.OldTime-b.NewTime) / float64(b.OldTime)
			fmt.Fprintf(w, "%s\t%d\t%d\t%.1f\n", b.Name, b.OldTime, b.NewTime, 100*sp)
		}
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
	Name      string
	Count     int64
	TimePerOp time.Duration
}

func runBenchmark(ctx context.Context, test string) ([]result, error) {
	out, err := exec.CommandContext(ctx, "go", "test", "-bench=.", "-run=^NONE", test).Output()
	if err != nil {
		return nil, err
	}
	var res []result
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		fields := strings.Fields(line)
		count, _ := strconv.ParseInt(fields[1], 10, 64)
		nsPerOp, _ := strconv.ParseInt(fields[2], 10, 64)
		res = append(res, result{
			Name:      fields[0],
			Count:     count,
			TimePerOp: time.Duration(nsPerOp) / time.Nanosecond,
		})
	}
	return res, nil
}

type joined struct {
	Name               string
	OldCount, NewCount int64
	OldTime, NewTime   time.Duration
}

func joinResults(old, new []result) []joined {
	var res []joined
	m := make(map[string]int)
	for _, b := range old {
		m[b.Name] = len(res)
		res = append(res, joined{
			Name:     b.Name,
			OldCount: b.Count,
			OldTime:  b.TimePerOp,
		})
	}
	for _, b := range new {
		if p, ok := m[b.Name]; ok {
			res[p].NewCount = b.Count
			res[p].NewTime = b.TimePerOp
		} else {
			res = append(res, joined{
				Name:     b.Name,
				NewCount: b.Count,
				NewTime:  b.TimePerOp,
			})
		}
	}
	return res
}
