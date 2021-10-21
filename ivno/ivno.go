package main

import (
	"flag"
	"fmt"
	"time"
)

var revNo = flag.Int("rev", 0, "Revision number (0 for initial)")

func main() {
	flag.Parse()
	ts := time.Now().In(time.UTC).Format("20060102150405")
	base := "S" + ts + "Z"
	if *revNo > 0 {
		fmt.Printf("%sR%d\n", base, *revNo)
	} else {
		fmt.Println(base)
	}
}
