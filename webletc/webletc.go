// Program webletc is a web browser bookmarklet "compiler".
// It packs, minifies, and encodes a JavaScript program into a URL in the
// javascript: scheme.
//
// Usage:
//    webletc input.js
//
// Output is written to stdout. If no input file is named, the tool reads
// source from stdin.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

func main() {
	flag.Parse()

	// Inject the input file into a wrapper:
	//
	//    void((()=>{ <SOURCE> })())
	//
	var src bytes.Buffer
	fmt.Fprint(&src, "void((()=>{")
	if err := readInput(&src); err != nil {
		log.Fatalf("Reading input: %v", err)
	}
	fmt.Fprint(&src, "})())")

	const mediaType = "application/javascript"
	m := minify.New()
	m.Add(mediaType, &js.Minifier{
		KeepVarNames: false,
	})

	var buf bytes.Buffer
	if err := m.Minify(mediaType, &buf, &src); err != nil {
		log.Fatalf("Minification failed: %v", err)
	}
	esc := strings.ReplaceAll(url.QueryEscape(buf.String()), "+", "%20")
	fmt.Println("javascript:" + esc)
}

func readInput(w io.Writer) error {
	if flag.NArg() == 0 {
		_, err := io.Copy(w, os.Stdin)
		return err
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
