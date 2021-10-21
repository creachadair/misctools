package main

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

var revNo = flag.Int("rev", 0, "Revision number (0 for initial)")

func main() {
	flag.Parse()
	var base string
	if flag.NArg() == 0 {
		ts := time.Now().In(time.UTC).Format("200601021504")
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
