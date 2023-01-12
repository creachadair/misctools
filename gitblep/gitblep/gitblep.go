// Package gitblep provides basic manipulation of Git objects in their binary
// format as stored in a repository.
package gitblep

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Repo represents a repository.
type Repo struct {
	Dir string
}

// Object loads the specified object from the repository.
func (r *Repo) Object(hash string) (*Object, error) {
	path := filepath.Join(r.Dir, "objects", hash[:2], hash[2:])
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj Object
	if err := obj.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	obj.Hash = hash
	return &obj, nil
}

// Object is the top-level wrapper for a Git object.
type Object struct {
	Hash string
	Type string
	Data []byte
}

func (o *Object) UnmarshalBinary(data []byte) error {
	blob, err := unzlib(data)
	if err != nil {
		return fmt.Errorf("uncompress: %w", err)
	}
	i := bytes.IndexByte(blob, 0)
	if i < 0 {
		return errors.New("object header not found")
	}
	o.Data = blob[i+1:]
	hdr := string(blob[:i])
	kind, size, ok := strings.Cut(hdr, " ")
	if !ok {
		return errors.New("invalid object header")
	}
	sz, err := strconv.Atoi(size)
	if err != nil {
		return fmt.Errorf("invalid object size %q: %w", size, err)
	} else if sz != len(o.Data) {
		return fmt.Errorf("wrong object size (have %d bytes, want %d)", len(o.Data), sz)
	}
	o.Type = kind
	return nil
}

// Commit is the representation of a commit object.
type Commit struct {
	Tree      string
	Parents   []string
	Author    ID
	Committer ID
	Log       string
}

func (c *Commit) UnmarshalBinary(data []byte) error {
	hdr, log, ok := strings.Cut(string(data), "\n\n")
	if !ok {
		return errors.New("invalid commit")
	}
	c.Log = log
	for _, line := range strings.Split(hdr, "\n") {
		tag, rest, _ := strings.Cut(line, " ")
		var err error
		switch tag {
		case "tree":
			c.Tree = rest
		case "parent":
			c.Parents = append(c.Parents, rest)
		case "author":
			err = c.Author.UnmarshalText([]byte(rest))
		case "committer":
			err = c.Committer.UnmarshalText([]byte(rest))
		default:
			return fmt.Errorf("invalid field: %q", tag)
		}
		if err != nil {
			return fmt.Errorf("invalid %s: %w", tag, err)
		}
	}
	return nil
}

// ID is the representation of a user ID (author, committer).
type ID struct {
	Name   string
	EMail  string
	Time   int64 // epoch time in seconds
	Offset int32 // timezone offset +/-HHMM
}

var idRE = regexp.MustCompile(`^(.*?) <(.*?)> (\d+) ([-+]\d{4})$`)

func (id *ID) UnmarshalText(data []byte) error {
	m := idRE.FindStringSubmatch(string(data))
	if m == nil {
		return errors.New("invalid ID spec")
	}
	id.Name = m[1]
	id.EMail = m[2]
	ts, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	off, err := strconv.ParseInt(m[4], 0, 32)
	if err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	id.Time = ts
	id.Offset = int32(off)
	return nil
}

// Tree is the representation of a tree object.
type Tree []Entry

func (t *Tree) UnmarshalBinary(data []byte) error {
	*t = nil

	const hashSize = sha1.Size

	i := 0
	for i < len(data) {
		j := bytes.IndexByte(data[i:], 0)
		if j < 0 || j+hashSize > len(data) {
			return fmt.Errorf("offset %d: incomplete entry", i)
		}
		end := i + 1 + j + hashSize
		var e Entry
		if err := e.UnmarshalBinary(data[i:end]); err != nil {
			return fmt.Errorf("entry %d: %w", len(*t)+1, err)
		}
		*t = append(*t, e)
		i = end
	}

	return nil
}

// An Entry represents a single element of a Tree.
type Entry struct {
	Mode uint32
	Type string
	Hash string
	Name string
}

func (e *Entry) UnmarshalBinary(data []byte) error {
	i := bytes.IndexByte(data, 0)
	if i < 0 {
		return errors.New("entry: missing separator")
	}
	mtext, name, _ := strings.Cut(string(data[:i]), " ")
	mode, err := strconv.ParseUint(mtext, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode: %w", err)
	}
	const modeDir = 040000
	if mode&modeDir != 0 {
		e.Type = "tree"
	} else {
		e.Type = "blob"
	}

	e.Name = name
	e.Mode = uint32(mode)
	e.Hash = fmt.Sprintf("%x", data[i+1:])
	return nil
}

func unzlib(data []byte) ([]byte, error) {
	zc, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zc.Close()
	return io.ReadAll(zc)
}
