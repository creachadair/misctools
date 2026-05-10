package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creachadair/atomicfile"
	"github.com/creachadair/command"
	"github.com/creachadair/mboxlib"
)

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
	return splitMailbox(f, func(s *mboxlib.Scanner, index int, msg *mboxlib.Message) error {
		line, _ := s.Lines()
		info := msgInfo{
			Index:       index,
			Line:        line,
			Size:        int64(len(msg.Data)),
			ContentType: msg.ParsedHeader.Get("Content-Type"),
		}
		if ctype, params, err := mime.ParseMediaType(info.ContentType); err == nil {
			info.ContentType = ctype
			info.Charset = params["charset"] // may be empty
			info.Boundary = params["boundary"]
		}
		return enc.Encode(info)
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

	return splitMailbox(f, func(s *mboxlib.Scanner, index int, msg *mboxlib.Message) error {
		mpath := filepath.Join(outDir, messagePath(msg))
		if err := os.MkdirAll(filepath.Dir(mpath), 0700); err != nil {
			return err
		}
		if err := atomicfile.WriteData(mpath, msg.Data, 0600); err != nil {
			return fmt.Errorf("write %q: %w", mpath, err)
		}
		line, _ := s.Lines()
		log.Printf("message %d (line %d) wrote %d bytes to %q", index, line, len(msg.Data), mpath)
		return nil
	})
}

func splitMailbox(r io.Reader, f func(s *mboxlib.Scanner, index int, msg *mboxlib.Message) error) error {
	s := mboxlib.NewScanner(r)
	var index int
	for {
		next, err := s.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		index++
		line, _ := s.Lines()

		msg, err := mboxlib.ParseMessage(next)
		if err != nil {
			log.Printf("WARNING: line %d: invalid message #%d: %v", line, index, err)
			continue
		}
		if err := f(s, index, msg); err != nil {
			return err
		}
	}
}

func messagePath(msg *mboxlib.Message) string {
	const nameStamp = "20060102-150405.999"
	tag, base := pickTag(msg), "0000/00"
	date, err := msg.ParsedHeader.Date()
	if err != nil {
		date = time.Now()
	} else {
		base = date.Format("2006/01")
	}
	name := fmt.Sprintf("msg.%s.txt", date.UTC().Format(nameStamp))
	return filepath.Join(tag, base, name)
}

func pickTag(msg *mboxlib.Message) string {
	tags := msg.ParsedHeader.Get("X-Gmail-Labels")
	btag := "misc"
	var score int
	for tag := range strings.SplitSeq(strings.ToLower(tags), ",") {
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
