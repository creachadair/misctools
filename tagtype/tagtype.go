// Program tagtype generates JSON marshaling and unmarshaling methods for
// designated types. It is intended for use from "go generate".
//
// The generated methods encode a value of the type as a JSON object with two
// fields:
//
//    {"type": "<type-tag>", "value": <json-encoded-value>}
//
// Types opt in to the generator by declaring a method that reports the desired
// string:
//
//    func (T) jsonWrapperTag() string { return "type/tag/for.T" }
//
// Types without this method are ignored.
//
// By default, both Marshal and Unmarshal methods are generated.
// Use the -m and -u flags to emit only one or the other.
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"sort"

	"github.com/creachadair/misctools/tagtype/gen"
)

var (
	outputPath   = flag.String("output", "", "Output file path (required)")
	inputDir     = flag.String("input", ".", "Input directory")
	genMarshal   = flag.Bool("m", false, "Generate marshal methods")
	genUnmarshal = flag.Bool("u", false, "Generate unmarshal methods")
)

func main() {
	flag.Parse()
	if *outputPath == "" {
		log.Fatal("You must provide a non-empty -output file path")
	}
	emitAll := !*genMarshal && !*genUnmarshal

	pkg, err := gen.Parse(*inputDir)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}
	names := gen.FindTypes(pkg)
	if len(names) == 0 {
		log.Fatalf("No matching types in package %q", pkg.Name)
	}

	sort.Strings(names)

	var buf bytes.Buffer
	gen.EmitFileHeader(&buf, pkg.Name)
	for _, name := range names {
		if emitAll || *genMarshal {
			gen.EmitMarshal(&buf, name)
		}
		if emitAll || *genUnmarshal {
			gen.EmitUnmarshal(&buf, name)
		}
	}

	f, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Creating output: %v", err)
	}
	if err := gen.FormatSource(f, buf.Bytes()); err != nil {
		f.Close()
		log.Fatalf("Formatting generated source: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("Closing output: %v", err)
	}
}
