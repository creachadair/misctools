// Program otpkey decodes base32-encoded OTP keys of the kind used to manually
// set up a Google authenticator.
package main

import (
	"encoding/base32"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s <base32-key>", filepath.Base(os.Args[0]))
	}

	key := strings.Join(flag.Args(), " ")
	clean := strings.ToUpper(strings.Join(strings.Fields(key), ""))
	dec, err := base32.StdEncoding.DecodeString(clean)
	if err != nil {
		log.Fatalf("Error decoding key %q: %v", clean, err)
	}
	fmt.Print(clean, "\t", base64.StdEncoding.EncodeToString(dec), "\n")
}
