package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/command"
)

var fromLineRE = regexp.MustCompile(`(?m)^From .*\n`)

type msgInfo struct {
	Index       int    `json:"index"`
	Line        int    `json:"line,omitempty"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Charset     string `json:"charset,omitempty"`
	Boundary    string `json:"boundary,omitempty"`
}

func runList(env *command.Env, mailbox string) error {
	f, err := os.Open(mailbox)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(os.Stdout)
	return splitMailbox(bufio.NewReader(f), func(index, lnum int, data []byte) error {
		_, mi, err := parseMessage(data)
		if err != nil {
			log.Printf("WARNING: index %d: invalid message: %v", index, err)
		}
		mi.Index = index
		mi.Line = lnum
		return enc.Encode(mi)
	})
}

func runBurst(env *command.Env, mailbox, outDir string) error {
	f, err := os.Open(mailbox)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return err
	}

	return splitMailbox(bufio.NewReader(f), func(index, lnum int, data []byte) error {
		msg, _, err := parseMessage(data)
		if err != nil {
			return fmt.Errorf("index %d line %d: %w", index, lnum, err)
		}
		mpath := filepath.Join(outDir, messagePath(msg))
		if err := os.MkdirAll(filepath.Dir(mpath), 0700); err != nil {
			return err
		}
		if err := atomicfile.WriteData(mpath, data, 0600); err != nil {
			return fmt.Errorf("write %q: %w", mpath, err)
		}
		log.Printf("message %d (line %d) wrote %d bytes to %q", index, lnum, len(data), mpath)
		return nil
	})
}

func splitMailbox(r io.Reader, f func(index, lnum int, msg []byte) error) error {
	var buf bytes.Buffer

	var atEOF bool
	index, lnum := 0, 1
	for {
		m := fromLineRE.FindIndex(buf.Bytes())
		if m == nil {
			if atEOF {
				break
			}
			var tmp [1 << 20]byte
			n, err := r.Read(tmp[:])
			buf.Write(tmp[:n])
			if err == io.EOF {
				atEOF = true
			} else if err != nil {
				return err
			}
			continue
		}

		msg := buf.Next(m[0])
		if len(msg) != 0 {
			index++
			if err := f(index, lnum, msg); err != nil {
				return err
			}
			lnum++ // for the From line itself
			lnum += bytes.Count(msg, []byte("\n"))
		}
		buf.Next(m[1] - m[0]) // drop the separator
	}

	if buf.Len() != 0 {
		index++
		return f(index, lnum, buf.Bytes())
	}
	return nil
}

func parseMessage(data []byte) (*mail.Message, msgInfo, error) {
	mi := msgInfo{Size: int64(len(data)), ContentType: "invalid"}
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err == nil {
		mi.ContentType = msg.Header.Get("Content-Type")
		ctype, params, err := mime.ParseMediaType(mi.ContentType)
		if err == nil {
			mi.ContentType = ctype
			mi.Charset = params["charset"]
			mi.Boundary = params["boundary"]
		}
	}
	return msg, mi, err
}

func messagePath(msg *mail.Message) string {
	const nameStamp = "20060102-150405.999"
	tag, base := pickTag(msg), "0000/00"
	date, err := msg.Header.Date()
	if err != nil {
		date = time.Now()
	} else {
		base = date.Format("2006/01")
	}
	name := fmt.Sprintf("msg.%s.txt", date.UTC().Format(nameStamp))
	return filepath.Join(tag, base, name)
}

func pickTag(msg *mail.Message) string {
	tags := msg.Header.Get("X-Gmail-Labels")
	btag := "misc"
	var score int
	for _, tag := range strings.Split(strings.ToLower(tags), ",") {
		got, sc := tagScore(tag)
		if sc > score {
			btag, score = got, sc
		}
	}
	return btag
}

func tagScore(t string) (string, int) {
	switch t {
	case "":
		return t, 0
	case "archived", "category", "opened", "sent":
		return t, 1
	case "spam":
		return t, 2
	case "[gmail]all mail":
		return "untagged", 2
	default:
		if rest, ok := strings.CutPrefix(t, "category "); ok {
			return rest, 3
		}
		return t, 4
	}
}
