// Program keyfile is a low-level command-line tool to read and write the
// contents of key files using the bitbucket.org/creachadair/keyfile package.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/getpass"
	"bitbucket.org/creachadair/keyfile"
	"bitbucket.org/creachadair/vocab"
	"golang.org/x/xerrors"
)

func main() {
	v, err := vocab.New(filepath.Base(os.Args[0]), new(kftool))
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := v.Dispatch(context.Background(), os.Args[1:]); err != nil {
		log.Fatalf("Run: %v", err)
	}
}

type getCmd struct{}

func (getCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return xerrors.New("usage: get <slug>")
	} else if !tool(ctx).File.Has(args[0]) {
		return xerrors.Errorf("get: no such key %q", args[0])
	}
	pp, err := tool(ctx).passphrase(ctx)
	if err != nil {
		return err
	}
	data, err := tool(ctx).File.Get(args[0], pp)
	if err != nil {
		return err
	}
	os.Stdout.Write(data)
	return nil
}

type randCmd struct {
	Len int `flag:"n,Length of generated key in bytes"`
}

func (r randCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return xerrors.New("usage: rand <slug>")
	} else if r.Len <= 0 {
		return xerrors.Errorf("rand: invalid length %d", r.Len)
	}
	pp, err := tool(ctx).passphrase(ctx)
	if err != nil {
		return err
	}
	data, err := tool(ctx).File.Random(args[0], pp, r.Len)
	if err != nil {
		return err
	}
	if err := tool(ctx).save(ctx); err != nil {
		return err
	}
	os.Stdout.Write(data)
	return nil
}

type setCmd struct{}

func (setCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return xerrors.New("usage: set <slug>")
	}
	pp, err := tool(ctx).passphrase(ctx)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	if err := tool(ctx).File.Set(args[0], pp, data); err != nil {
		return err
	}
	return tool(ctx).save(ctx)
}

type delCmd struct{}

func (delCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return xerrors.New("usage: del <slug>")
	} else if tool(ctx).File.Remove(args[0]) {
		return tool(ctx).save(ctx)
	}
	fmt.Fprintln(os.Stderr, "No change")
	return nil
}

type listCmd struct{}

func (listCmd) Run(ctx context.Context, args []string) error {
	if s := tool(ctx).File.Slugs(); len(s) != 0 {
		fmt.Println(strings.Join(s, "\n"))
	}
	return nil
}

type jsonCmd struct{}

func (jsonCmd) Run(ctx context.Context, args []string) error {
	var buf bytes.Buffer
	if err := tool(ctx).File.WriteJSON(&buf); err != nil {
		return err
	}
	fmt.Println(buf.String())
	return nil
}

type kftool struct {
	Path string `flag:"f,Path of key file (required)"`
	File *keyfile.File

	Help vocab.Help `vocab:"help"`
	Del  delCmd     `vocab:"remove,delete,del,rm" help-summary:"Delete the specified key"`
	Get  getCmd     `vocab:"get" help-summary:"Write the specified key to stdout"`
	JSON jsonCmd    `vocab:"json" help-summary:"Write the keyfile as JSON to stdout"`
	List listCmd    `vocab:"list,ls" help-summary:"List the key slugs in the keyfile"`
	Rand randCmd    `vocab:"rand" help-summary:"Generate and store a random key of a specified length"`
	Set  setCmd     `vocab:"set" help-summary:"Store the contents of stdin as a key"`

	_ struct{} `help-summary:"A tool to read and write keyfiles"`
}

type rootKey struct{}

func tool(ctx context.Context) *kftool { return ctx.Value(rootKey{}).(*kftool) }

func (k *kftool) Init(ctx context.Context, name string, args []string) (context.Context, error) {
	if name == "help" {
		return ctx, nil
	} else if k.Path == "" {
		return nil, xerrors.New("missing key file path (-f)")
	}

	f, err := os.Open(k.Path)
	if os.IsNotExist(err) {
		k.File = keyfile.New()
		return context.WithValue(ctx, rootKey{}, k), nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	kf, err := keyfile.Load(f)
	if err != nil {
		return nil, err
	}
	k.File = kf
	return context.WithValue(ctx, rootKey{}, k), nil
}

func (k *kftool) save(ctx context.Context) error {
	f, err := os.Create(k.Path)
	if err != nil {
		return err
	}
	_, err = k.File.WriteTo(f)
	cerr := f.Close()
	if err != nil {
		return err
	}
	return cerr
}

func (k *kftool) passphrase(ctx context.Context) (string, error) {
	return getpass.Prompt("Passphrase: ")
}
