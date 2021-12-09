package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	revNo   = flag.Int("rev", 0, "Revision number (0 for initial)")
	useDate = flag.String("date", "", "Use this date (YYYY-MM-DD)")
)

func main() {
	flag.Parse()
	var base string
	if flag.NArg() == 0 {
		ts := timestamp().Format("200601021504")
		base = "S" + ts + "Z"
	} else {
		base = strings.SplitN(flag.Arg(0), "R", 2)[0]
	}
	if *revNo > 0 {
		fmt.Printf("%sR%d\n", base, *revNo)
	} else {
		fmt.Println(base)
	}
}

func timestamp() time.Time {
	ts := time.Now().In(time.UTC)
	if *useDate != "" {
		d, err := time.Parse("2006-01-02", *useDate)
		if err != nil {
			log.Fatalf("Invalid date: %v", err)
		}
		return time.Date(d.Year(), d.Month(), d.Day(),
			ts.Hour(), ts.Minute(), ts.Second(), ts.Nanosecond(), time.UTC)
	}
	return ts
}
