// Program stockopt optimizes a stock sale subject to limitations of capital
// gains.  The input to the program is an .xls spreadsheet as generated from
// the Gain/Loss view of the MSSB stock plan site. The output is a table
// listing how many of each lot of stock should be sold, the total sale price
// based on the estimated sale values from MSSB, and the total capital gain
// from the sale.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"time"

	"local/currency"
	"local/statement"
)

var (
	inputPath    = flag.String("input", "", "Input file (.xls)")
	ageMonths    = flag.Int("age", 12, "Minimum age in months")
	planFilter   = flag.String("plan", "GSU Class C", "View only this plan")
	capGainLimit = flag.String("cap", "$25000", "Capital gain limit in USD")
	printSummary = flag.Bool("summary", false, "Print summary of available shares")
	allowLoss    = flag.Bool("loss", false, "Allow sale of capital losses")
)

func main() {
	flag.Parse()
	if *inputPath == "" {
		log.Fatal("You must provide an -input .xls path")
	}

	// Convert the capital gains cap into a currency value.
	maxGain, err := currency.ParseUSD(*capGainLimit)
	if err != nil {
		log.Fatalf("Invalid cap %q: %v", *capGainLimit, err)
	}

	// Read and parse the input spreadsheet, filtering out entries with 0
	// available shares, those issued more recently than the specified age, and
	// not matching the specified plan filter.
	data, err := ioutil.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("Reading statement: %v", err)
	}

	then := time.Now().AddDate(0, -*ageMonths, 0)
	es, err := statement.Parse(data, func(e *statement.Entry) bool {
		return e.Available > 0 && e.Acquired.Before(then) &&
			(*planFilter == "" || e.Plan == *planFilter) &&
			(e.Gain >= 0 || *allowLoss)
	})
	if err != nil {
		log.Fatalf("Parsing statement: %v", err)
	}

	// Compute the total value of the portfolio, just for cosmetics.
	var totalValue, totalGain currency.Value
	var totalShares int
	for _, e := range es {
		totalShares += e.Available
		v := currency.Value(e.Available)
		totalValue += v * e.Value
		totalGain += v * e.Gain
	}

	fmt.Printf(`Input file:   %q
Minimum age:  %d months
Gains cap:    %s
Allow loss:   %v
Total shares: %d
Total value:  %s
Total gains:  %s

`, *inputPath, *ageMonths, maxGain.USD(), *allowLoss, totalShares, totalValue.USD(), totalGain.USD())

	// If requested, print a summary of available shares.
	if *printSummary {
		fmt.Println("Available shares:")
		for _, e := range es {
			fmt.Printf("%d.\t%s\n", e.Index, e.Format(-1))
		}
		fmt.Println()
	}

	// Order shares increasing by capital gains.  The share value should be
	// fixed at a given point in time, so we are trying to maximize number sold
	// subject to the cap. Thus, by considering gains in nondecreasing order we
	// ensure we sell as many as possible.
	sort.Slice(es, func(i, j int) bool {
		return es[i].Gain < es[j].Gain
	})

	type sale struct {
		count int
		entry *statement.Entry
	}
	var sell []sale
	var soldValue, soldGains currency.Value
	var soldShares int
	for _, e := range es {
		num := int((maxGain - soldGains) / e.Value)
		if num == 0 {
			break
		} else if num > e.Available {
			num = e.Available
		}
		soldShares += num
		soldValue += currency.Value(num) * e.Value
		soldGains += currency.Value(num) * e.Gain
		sell = append(sell, sale{
			count: num,
			entry: e,
		})
	}

	fmt.Printf("Sold shares:  %d\nSold value:   %s\nSold gains:   %s\n\n",
		soldShares, soldValue.USD(), soldGains.USD())

	// Put the results in a useful order.
	sort.Slice(sell, func(i, j int) bool {
		return sell[i].entry.Acquired.Before(sell[j].entry.Acquired)
	})

	for _, s := range sell {
		fmt.Printf("Sell [lot %2d]: %s\n", s.entry.Index, s.entry.Format(s.count))
	}
}
