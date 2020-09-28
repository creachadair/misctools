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
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

var (
	doData = flag.Bool("data", false, "Encode as a data URL")
	noWrap = flag.Bool("nowrap", false, "Omit the scope wrapper")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] [input-file]

"Compile" an input JavaScript text into a URL. Compiling consists of
minification and encoding. If an input file is not specified, source
is read from stdin.

By default, a javascript: URL is produced; Use -data to generate a
URL in the data: scheme instead.

By default, the output is wrapped in a scope so that its names do not
pollute the global namespace. Use -nowrap to disable this wrapping.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Inject the input file into a wrapper:
	//
	//    void((()=>{ <SOURCE> })())
	//
	var src bytes.Buffer
	if !*noWrap {
		fmt.Fprint(&src, "void((()=>{")
	}
	if err := readInput(&src); err != nil {
		log.Fatalf("Reading input: %v", err)
	}
	if !*noWrap {
		fmt.Fprint(&src, "})())")
	}

	const mediaType = "application/javascript"
	m := minify.New()
	m.Add(mediaType, &js.Minifier{
		KeepVarNames: false,
	})

	var buf bytes.Buffer
	if err := m.Minify(mediaType, &buf, &src); err != nil {
		log.Fatalf("Minification failed: %v", err)
	}

	if *doData {
		esc := base64.URLEncoding.EncodeToString(buf.Bytes())
		fmt.Println("data:" + mediaType + ";base64," + esc)
	} else {
		esc := strings.ReplaceAll(url.QueryEscape(buf.String()), "+", "%20")
		fmt.Println("javascript:" + esc)
	}
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
