// Program retry repeatedly invokes a subcommand, with exponential backoff,
// until it succeeds.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var (
	doRepeat  = flag.Bool("repeat", false, "After successful execution, run the command again")
	minPoll   = flag.Duration("min", 500*time.Millisecond, "Minimum poll interval")
	maxPoll   = flag.Duration("max", 1*time.Minute, "Maximum poll interval")
	pauseTime = flag.Duration("pause", 0, "Time to pause after a successful invocation")
	beQuiet   = flag.Bool("quiet", false, "Suppress log output")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [options] <command> <args>...

Repeatedly invoke the given command and arguments until it succeeds.  

On error, retry pauses temporarily (with exponential backoff up to --max). and
tries again.  Errors in starting up the command (for example, due to a missing
program) are not retried.

If --repeat is set, the command is rerun after each successful completion, with
an optional delay specified by --pause.

Options:
`, os.Args[0])
		flag.PrintDefaults()
	}

	log.SetOutput(os.Stderr)
}

const (
	exitDone    = 0 // command complete
	exitStartup = 1 // error starting up the command
)

func main() {
	flag.Parse()
	switch {
	case *minPoll < 10*time.Millisecond:
		log.Fatalf("Poll interval must be at least 10ms: %v", *minPoll)
	case *maxPoll < *minPoll:
		log.Fatalf("Maximum polling interval is less than minimum: %v < %v", *maxPoll, *minPoll)
	case flag.NArg() == 0:
		log.Fatal("You must provide a command to execute")
	}
	os.Exit(run(context.Background()))
}

func logPrintf(msg string, args ...interface{}) {
	if !*beQuiet {
		log.Printf(msg, args...)
	}
}

func run(ctx context.Context) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// When signalled, cancel the context so that the subprocess also gets
	// terminated cleanly before shutting down.
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case s := <-sig:
			logPrintf("Received %v signal; stopping...", s)
			cancel()
		}
	}()

	cur := *minPoll
	for {
		cmd := exec.CommandContext(ctx, flag.Arg(0), flag.Args()[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Try starting the command. If starting the command fails, or if the
		// command exits unsuccessfully, wait and try again.
		if err := cmd.Start(); err != nil {
			logPrintf("ERROR: Starting %q command failed: %v", flag.Arg(0), err)
			return exitStartup
		}

		// Tripping the signal handler will kill the subprocess, causing the
		// Wait call to report an error.
		var waitFor time.Duration
		if err := cmd.Wait(); err != nil {
			logPrintf("ERROR: Command %q failed: %v", flag.Arg(0), err)
			waitFor = cur
			cur *= 2
			if cur > *maxPoll {
				cur = *maxPoll
			}
		} else if !*doRepeat {
			return exitDone // success, retries disabled
		} else {
			cur = *minPoll // reset poll time since we succeeded
			waitFor = *pauseTime
		}

		select {
		case <-ctx.Done():
			return exitDone

		case <-time.After(waitFor):
			// try again...
		}
	}
}
