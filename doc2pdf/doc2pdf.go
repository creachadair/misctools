// Program doc2pdf uses the Pages application on macOS (via AppleScript) to
// convert .doc files from Microsoft Word into PDF format.
//
// This requires that the "osascript" command line tool is installed and has
// permission to execute scripts on behalf of the user. It also requies that
// the Pages application is installed.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

var dryRun = flag.Bool("dry-run", false, "Print the conversion script and exit")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] docfile...

Convert Microsoft Word .doc files to .pdf. This tool uses the macOS Pages
application and AppleScript to do the conversion, so it will not work on other
systems. Filenames not ending in .doc are skipped.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	script, paths, err := compile(flag.Args())
	if err != nil {
		log.Fatalf("Compiling script: %v", err)
	}
	if *dryRun {
		fmt.Fprintln(os.Stderr, script.(*bytes.Buffer).String())
		return
	} else if len(paths) == 0 {
		log.Fatal("Nothing to do")
	}

	if err := convert(context.Background(), script); err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	for _, path := range paths {
		fmt.Print(path.Src, "\t", path.Dst, "\n")
	}
}

var script = template.Must(template.New("convert").Parse(`-- Convert .doc files to .pdf
tell application "Pages"
{{range .}}
	open file "{{.Src}}" as POSIX file
	export document 1 to (file "{{.Dst}}" as POSIX file) as PDF
	close document 1
{{end}}
end tell
`))

type pathInfo struct{ Src, Dst string }

func compile(args []string) (io.Reader, []pathInfo, error) {
	var info []pathInfo
	for _, path := range args {
		src, err := filepath.Abs(path)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving %q: %v", path, err)
		}
		ext := filepath.Ext(src)
		if ext != ".doc" {
			log.Printf("Warning: skipped path %q with extension %q", path, ext)
			continue
		}
		dst := strings.TrimSuffix(src, ext) + ".pdf"
		info = append(info, pathInfo{Src: src, Dst: dst})
	}
	var buf bytes.Buffer
	if err := script.Execute(&buf, info); err != nil {
		return nil, nil, err
	}
	return &buf, info, nil
}

func convert(ctx context.Context, script io.Reader) error {
	cmd := exec.CommandContext(ctx, "osascript")
	cmd.Stdin = script
	_, err := cmd.Output()
	if e, ok := err.(*exec.ExitError); ok {
		return errors.New(string(e.Stderr))
	}
	return err
}
