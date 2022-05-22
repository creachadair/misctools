// Program fileblit destructively moves a file from one path to another.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	inPath    = flag.String("in", "", "Input file path")
	outPath   = flag.String("out", "", "Output file path")
	blockSize = flag.Int64("block", 1<<20, "Transfer block size")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s -in src -out dst

Destructively move a file from -in to -out. Unlike the "mv" command, this
command does not copy the entire file and then unlink the source, but moves
the file contents block-by-block to the output and deletes them from the input.

This approach is useful is if you need to move a large file to another volume
on the same storage but do not have enough space for multiple copies.

The move works from the end toward the beginning, copying the last block and
then truncating the input to remove that block. The copy does not touch parts
of the output file past the end of the input, so it is safe to resume a copy
that was interrupted (as long as the output was not modified separately).

After a complete move, the input file will be empty.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	switch {
	case *inPath == "":
		log.Fatal("You must provide a non-empty -in file path")
	case *outPath == "":
		log.Fatal("You must provide a non-empty -out file path")
	case *inPath == *outPath:
		log.Fatalf("The -in and -out paths must differ: %q", *inPath)
	}

	in, err := os.OpenFile(*inPath, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Input file: %v", err)
	}
	ifs, err := in.Stat()
	if err != nil {
		log.Fatalf("Input stat: %v", err)
	}
	log.Printf("Input file is %d bytes", ifs.Size())

	out, err := os.OpenFile(*outPath, os.O_RDWR|os.O_CREATE, ifs.Mode())
	if err != nil {
		log.Fatalf("Output file: %v", err)
	}

	buf := make([]byte, *blockSize)

	end := ifs.Size()
	for end > 0 {
		pos := end - *blockSize
		if pos < 0 {
			pos = 0
		}
		if _, err := in.Seek(pos, io.SeekStart); err != nil {
			log.Fatalf("Seek input %d failed: %v", pos, err)
		}
		if _, err := out.Seek(pos, io.SeekStart); err != nil {
			log.Fatalf("Seek output %d failed: %v", pos, err)
		}

		block := buf[:end-pos]
		if _, err := io.ReadFull(in, block); err != nil {
			log.Fatalf("Read %d bytes at %d: %v", len(buf), pos, err)
		}
		if _, err := out.Write(block); err != nil {
			log.Fatalf("Write %d bytes at %d: %v", len(buf), pos, err)
		}
		if err := in.Truncate(pos); err != nil {
			log.Fatalf("Truncate input at %d: %v", pos, err)
		}

		fmt.Printf("%d %d OK\n", end, len(block))
		end = pos
	}
	fmt.Printf("%d DONE\n", end)
	if err := out.Sync(); err != nil {
		log.Printf("Warning: sync output: %v", err)
	}

	ierr := in.Close()
	oerr := out.Close()
	if ierr != nil {
		log.Printf("Close input: %v", err)
	}
	if oerr != nil {
		log.Printf("Close output: %v", err)
	}
	if ierr != nil || oerr != nil {
		os.Exit(1)
	}
}
