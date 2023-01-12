package gitblep

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type ObjType byte

const (
	ObjInvalid     ObjType = 0
	ObjCommit      ObjType = 1
	ObjTree        ObjType = 2
	ObjBlob        ObjType = 3
	ObjTag         ObjType = 4
	ObjOffsetDelta ObjType = 6
	ObjRefDelta    ObjType = 7

	// Type 5 is reserved.
)

var otype = [...]string{
	ObjInvalid:     "<invalid>",
	ObjCommit:      "commit",
	ObjTree:        "tree",
	ObjBlob:        "blob",
	ObjTag:         "tag",
	ObjOffsetDelta: "offset-delta",
	ObjRefDelta:    "ref-delta",
}

func (t ObjType) String() string {
	v := int(t)
	if v == 5 || v < 0 || v > len(otype) {
		return fmt.Sprintf("%s:%d", otype[ObjInvalid], v)
	}
	return otype[v]
}

type Index struct {
}

func (r *Repo) Pack(hash string) (*Pack, error) {
	path := filepath.Join(r.Dir, "objects", "pack", "pack-"+hash+".pack")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p := &Pack{Path: path}
	if _, err := p.ReadFrom(f); err != nil {
		return nil, err
	}
	return p, nil
}

type Pack struct {
	Path       string
	Version    uint32
	NumObjects uint32
	Chunks     []Chunk
}

func (p *Pack) ReadFrom(r io.Reader) (int64, error) {
	rd := bufio.NewReader(r)
	var pos int64

	var magic [4]byte
	nr, err := io.ReadFull(rd, magic[:])
	pos += int64(nr)
	if err != nil {
		return pos, err
	} else if s := string(magic[:]); s != "PACK" {
		return pos, fmt.Errorf("invalid magic %q", s)
	}

	nr, err = readUint32(rd, &p.Version)
	pos += int64(nr)
	if err != nil {
		return pos, err
	}
	nr, err = readUint32(rd, &p.NumObjects)
	pos += int64(nr)
	if err != nil {
		return pos, err
	}

	for {
		var size uint64
		var otype ObjType

		nr, otype, size, err = readSize(rd)
		pos += int64(nr)
		if err != nil {
			if err == io.EOF && nr == 0 {
				break
			}
			return pos, err
		}
		p.Chunks = append(p.Chunks, Chunk{
			Offset: pos,
			Size:   int64(size),
			Type:   otype,
		})
		log.Printf("MJF :: pos=%d size=%d type=%v", pos, size, otype)

		// Now we need to read past the data...
		nc, err := io.Copy(io.Discard, io.LimitReader(rd, int64(size)))
		pos += nc
		if err != nil {
			return pos, err
		} else if nc != int64(size) {
			return pos, fmt.Errorf("chunk truncated at offset %d (got %d, want %d)",
				pos, nc, size)
		}
	}
	return pos, nil
}

type Chunk struct {
	Offset int64
	Size   int64
	Type   ObjType
}

func readSize(r io.ByteReader) (int, ObjType, uint64, error) {
	var size uint64
	var otype ObjType

	nr, more, shift := 0, true, 4
	for more {
		b, err := r.ReadByte()
		if err != nil {
			return nr, 0, 0, err
		}

		// The first three bits of the first byte are a type tag.
		if nr == 0 {
			otype = ObjType((b >> 4) & 0x7)
			size = uint64((b >> 4) & 0xf)
		} else {
			size |= uint64(b&0x7f) << shift
			shift += 7
		}
		nr++
		more = b&0x80 != 0
		log.Printf("MJF :: nr=%d b=%x more=%v", nr, b, more)
	}
	return nr, otype, size, nil
}

func readUint32(r io.Reader, v *uint32) (int, error) {
	var buf [4]byte
	nr, err := io.ReadFull(r, buf[:])
	if err != nil {
		return nr, err
	}
	*v = binary.BigEndian.Uint32(buf[:])
	return len(buf), nil
}
