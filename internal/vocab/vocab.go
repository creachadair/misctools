// Copyright (C) 2019 Michael J. Fromberger. All Rights Reserved.

// Package vocab handles flag parsing and dispatch for a nested language of
// commands and subcommands. In this model, a command-line is treated as a
// phrase in a simple grammar:
//
//    command = name [flags] [command]
//
// Each name may be either a command in itself, or a group of subcommands with
// a shared set of flags, or both.
//
// You describe command vocabulary with nested struct values, whose fields
// define flags and subcommands to be executed.  The implementation of a
// command is provided by by implementing the vocab.Runner interface. Commands
// may pass shared state to their subcommands by attaching it to a context
// value that is propagated down the vocabulary tree.
//
// Basic usage outline:
//
//    itm, err := vocab.New("toolname", v)
//    ...
//    if err := itm.Dispatch(ctx, args); err != nil {
//       log.Fatalf("Dispatch failed: %v, err)
//    }
//
package vocab

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"bitbucket.org/creachadair/stringset"
	"golang.org/x/xerrors"
)

// A Runner executes the behaviour of a command. If a command implements the
// Run method, it will be used to invoke the command after flag parsing.
type Runner interface {
	// Run executes the command with the specified arguments.
	//
	// The context passed to run contains any values attached by the Init
	// methods of enclosing commands.
	Run(ctx context.Context, args []string) error
}

// RunFunc implements the vocab.Runner interface by calling a function with the
// matching signature. This can be used to embed command implementations into
// the fields of a struct type with corresponding signatures.
type RunFunc func(context.Context, []string) error

// Run satisfies the vocab.Runner interface.
func (rf RunFunc) Run(ctx context.Context, args []string) error { return rf(ctx, args) }

// An Initializer sets up the environment for a subcommand. If a command
// implements the Init method, it will be called before dispatching control to
// a subcommand.
type Initializer interface {
	// Init prepares a command for execution of the named subcommand with the
	// given arguments, prior to parsing the subcommand's flags. The name is the
	// resolved canonical name of the subcommand, and the first element of args
	// is the name as written (which may be an alias).
	//
	// If the returned context is not nil, it replaces ctx in the subcommand;
	// otherwise ctx is used. If init reports an error, the command execution
	// will fail.
	Init(ctx context.Context, name string, args []string) (context.Context, error)
}

// New constructs a vocabulary item from the given value. The root value must
// either itself implement the vocab.Runner interface, or be a (pointer to a)
// struct value whose field annotations describe subcommand vocabulary.
//
// To define a field as implementing a subcommand, use the "vocab:" tag to
// define its name:
//
//    type Cmd struct{
//       A Type1  `vocab:"first"`
//       B *Type2 `vocab:"second"`
//    }
//
// The field types in this example must similarly implement vocab.Runner, or be
// structs with their own corresponding annotations.  During dispatch, an
// argument list beginning with "first" will dispatch through A, and an
// argument list beginning with "second" will dispatch through B.  The nesting
// may occur to arbitrary depth, but note that New does not handle cycles.
//
// A subcommand may also have aliases, specified as:
//
//    vocab:"name,alias1,alias2,..."
//
// The names and aliases must be unique within a given value.
//
// You can also attach flag to struct fields using the "flag:" tag:
//
//    flag:"name,description"
//
// The name becomes the flag string, and the description its help text.  The
// field must either be one of the standard types understood by the flag
// package, or its pointer must implement the flag.Value interface. In each
// case the default value for the flag is the current value of the field.
//
// Documentation
//
// In addition to its name, each command has "summary" and "help" strings. The
// summary is a short (typically one-line) synopsis, and help is a longer and
// more explanatory (possibly multi-line) description. There are three ways to
// associate these strings with a command:
//
// If the command implements vocab.Summarizer, its Summary method is used to
// generate the summary string.  Otherwise, if the command has a blank field
// ("_") whose tag begins with "help-summary:", the rest of that tag is used as
// the summary string. Otherwise, if the command's type is used as a field of
// an enclosing command type with a "help-summary:" comment tag, that text is
// used as the summary string for the command.
//
// If the type implements vocab.Helper, its Help method is used to generate the
// full help string.  Otherwise, if the command has a blank field ("_") whose
// tag begins with "help-long:", the rest of that tag is used as the long help
// string. Otherwise, if the command's type is used as a field of an enclosing
// command type with a "help-log:" comment tag, that text is used as the long
// help string for the command.
//
// Caveat: Although the Go grammar allows arbitrary string literals as struct
// field tags, there is a strong convention supported by the reflect package
// and "go vet" for single-line tags with key:"value" structure. This package
// will accept multi-line unquoted tags, but be aware that some lint tools may
// complain if you use them. You can use standard string escapes (e.g., "\n")
// in the quoted values of tags to avoid this, at the cost of a long line.
func New(name string, root interface{}) (*Item, error) {
	itm, err := newItem(name, root)
	if err != nil {
		return nil, err
	} else if itm.init == nil && itm.run == nil && len(itm.items) == 0 {
		return nil, xerrors.New("value does not implement any vocabulary")
	}
	return itm, nil
}

// An Item represents a parsed command tree.
type Item struct {
	cmd  interface{}
	run  func(context.Context, []string) error
	init func(context.Context, string, []string) (context.Context, error)

	name     string            // the canonical name of this command
	summary  func() string     // the summary text, if defined
	helpText func() string     // the long help text, if defined
	fs       *flag.FlagSet     // the flags for the command (if nil, none are defined)
	hasFlags bool              // whether any flags were explicitly defined
	items    map[string]*Item  // :: name → subcommand
	alias    map[string]string // :: subcommand alias → name
	out      io.Writer         // output writer
}

// SetOutput sets the output writer for m and all its nested subcommands to w.
// It will panic if w == nil.
func (m *Item) SetOutput(w io.Writer) {
	if w == nil {
		panic("vocab: output writer is nil")
	}
	m.out = w
	m.fs.SetOutput(w)
	for _, itm := range m.items {
		itm.SetOutput(w)
	}
}

// shortHelp prints a brief summary of m to its output writer.
func (m *Item) shortHelp() { m.printHelp(false) }

// longHelp prints complete help for m to its output writer.
func (m *Item) longHelp() { m.printHelp(true) }

// printHelp prints a summary of m to w, including help text if full == true.
func (m *Item) printHelp(full bool) {
	w := m.out

	// Always print a summary line giving the command name.
	summary := "(undocumented)"
	if m.summary != nil {
		summary = m.summary()
	}
	fmt.Fprint(w, m.name, ": ", summary, "\n")

	// If full help is requested, include help text and flags.
	if full {
		if m.helpText != nil {
			fmt.Fprint(w, "\n", m.helpText(), "\n")
		}
		if m.hasFlags {
			fmt.Fprintln(w, "\nFlags:")
			m.fs.SetOutput(w)
			m.fs.PrintDefaults()
		}
	}

	// If any subcommands are defined, summarize them.
	if len(m.items) != 0 {
		amap := make(map[string][]string)
		for alias, name := range m.alias {
			amap[name] = append(amap[name], alias)
		}

		fmt.Fprintln(w, "\nSubcommands:")
		tw := tabwriter.NewWriter(w, 0, 8, 3, ' ', 0)
		for _, name := range stringset.FromKeys(m.items).Elements() {
			summary := "(undocumented)"
			if s := m.items[name].summary; s != nil {
				summary = s()
			}
			fmt.Fprint(tw, "  ", name, "\t", summary)

			// Include aliases in the summary, if any exist.
			if as := amap[name]; len(as) != 0 {
				sort.Strings(as)
				fmt.Fprintf(tw, " (alias: %s)", strings.Join(as, ", "))
			}
			fmt.Fprintln(tw)
		}
		tw.Flush()
	}
}

// findCommand reports whether key is the name or registered alias of any
// subcommand of m, and if so returns its item.
func (m *Item) findCommand(key string) (*Item, bool) {
	itm, ok := m.items[key]
	if !ok {
		itm, ok = m.items[m.alias[key]]
	}
	return itm, ok
}

// Resolve traverses the vocabulary of m to find the command described by path.
// The path should contain only names; Resolve does not parse flags.
// It returns the last item successfully reached, along with the unresolved
// tail of the path (which is empty if the path was fully resolved).
func (m *Item) Resolve(path []string) (*Item, []string) {
	cur := m
	for i, arg := range path {
		next, ok := cur.findCommand(arg)
		if !ok {
			return cur, path[i:]
		}
		cur = next
	}
	return cur, nil
}

// Dispatch traverses the vocabulary of m parsing and executing the described
// commands.
func (m *Item) Dispatch(ctx context.Context, args []string) error {
	if err := m.fs.Parse(args); err == flag.ErrHelp {
		return nil // the usage message already contains the short help
	} else if err != nil {
		return xerrors.Errorf("parsing flags for %q: %w", m.name, err)
	} else {
		args = m.fs.Args()
	}

	// Check whether there is a subcommand that can follow from m.
	if len(args) != 0 {
		if sub, ok := m.findCommand(args[0]); ok {
			// Having found a subcommand, give m a chance to initialize itself and
			// update the context before invoking the subcommand.
			if m.init != nil {
				pctx, err := m.init(ctx, sub.name, args)
				if err != nil {
					return xerrors.Errorf("subcommand %q: %w", m.name, err)
				} else if pctx != nil {
					ctx = pctx
				}
			}

			// Dispatch to the subcommand with the remaining arguments.
			return sub.Dispatch(withParent(ctx, m), args[1:])
		}

		// No matching subcommand; fall through and let this command handle it.
	}

	if m.run != nil {
		return m.run(ctx, args)
	} else if len(args) != 0 {
		return xerrors.Errorf("no command found matching %q", args)
	}
	m.shortHelp()
	return nil // TODO: Return something like flag.ErrHelp
}

// parentItemKey identifies the parent of a subcommand in the context.  This is
// used by the help system to locate the item for which help is requested, and
// is not exposed outside the package.
type parentItemKey struct{}

func withParent(ctx context.Context, m *Item) context.Context {
	return context.WithValue(ctx, parentItemKey{}, m)
}

func parentItem(ctx context.Context) *Item {
	if v := ctx.Value(parentItemKey{}); v != nil {
		return v.(*Item)
	}
	return nil
}

func newItem(name string, x interface{}) (*Item, error) {
	// Requirements: x must either be a struct or a non-nil pointer to a struct,
	// or must implement the vocab.Runnier interface.
	r, isRunner := x.(Runner)
	v := reflect.Indirect(reflect.ValueOf(x))
	isStruct := v.Kind() == reflect.Struct
	if !isRunner && !isStruct {
		return nil, xerrors.Errorf("value must be a struct (have %v)", v.Kind())
	}

	item := &Item{
		cmd:   x,
		name:  name,
		fs:    flag.NewFlagSet(name, flag.ContinueOnError),
		items: make(map[string]*Item),
		alias: make(map[string]string),
		out:   os.Stderr,
	}
	item.fs.Usage = func() { item.shortHelp() }

	// Cache the callbacks, if they are defined.
	if isRunner {
		item.run = r.Run
	}
	if in, ok := x.(Initializer); ok {
		item.init = in.Init
	}
	if sum, ok := x.(Summarizer); ok {
		item.summary = sum.Summary
	}
	if help, ok := x.(Helper); ok {
		item.helpText = help.Help
	}

	if !isStruct {
		return item, nil // nothing more to do here
	}

	// At this point we know x is a struct.  Scan its field tags for
	// annotations.
	//
	// To indicate that the field is a subcommand:
	//   vocab:"name" or vocab:"name,alias,..."
	//
	// To attach a flag to a field:
	//   flag:"name,description"
	//
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i) // field type metadata (name, tags)
		fv := v.Field(i) // field value

		// Annotation: flag:"name,description".
		// This requires x is a pointer.
		if tag := ft.Tag.Get("flag"); tag != "" && ft.PkgPath == "" {
			if fv.Kind() != reflect.Ptr {
				if !fv.CanAddr() {
					return nil, xerrors.Errorf("cannot flag field %q of type %T", ft.Name, x)
				}
				fv = fv.Addr()
			} else if !fv.Elem().IsValid() {
				return nil, xerrors.Errorf("cannot flag pointer field %q with nil value", ft.Name)
			}
			fname, help := tag, tag
			if i := strings.Index(tag, ","); i >= 0 {
				fname, help = tag[:i], tag[i+1:]
			}
			if err := registerFlag(item.fs, fv.Interface(), fname, help); err != nil {
				return nil, xerrors.Errorf("flagged field %q: %w", ft.Name, err)
			}
			item.hasFlags = true
			continue
		}

		// Annotation: vocab:"name" or vocab:"name,alias1,alias2,..."
		if tag := ft.Tag.Get("vocab"); tag != "" {
			names := strings.Split(tag, ",")
			if fv.Kind() != reflect.Ptr && fv.CanAddr() {
				fv = fv.Addr()
			}
			if !fv.CanInterface() {
				return nil, xerrors.Errorf("vocab field %q: cannot capture unexported value", ft.Name)
			}
			sub, err := newItem(names[0], fv.Interface())
			if err != nil {
				return nil, xerrors.Errorf("vocab field %q: %w", ft.Name, err)
			} else if _, ok := item.items[sub.name]; ok {
				return nil, xerrors.Errorf("duplicate subcommand %q", name)
			}

			// If the field returned a command but lacks documentation, check for
			// help tags on the field.
			if sub.summary == nil {
				if hs := ft.Tag.Get("help-summary"); hs != "" {
					sub.summary = func() string { return hs }
				}
			}
			if sub.helpText == nil {
				if hs := ft.Tag.Get("help-long"); hs != "" {
					sub.helpText = func() string { return hs }
				}
			}

			item.items[sub.name] = sub
			for _, a := range names[1:] {
				if old, ok := item.alias[a]; ok && old != sub.name {
					return nil, xerrors.Errorf("duplicate alias %q (%q, %q)", a, old, sub.name)
				}
				item.alias[a] = sub.name
			}
			continue
		}

		// Check for help annotations embedded in blank fields. Note that we do
		// not require these tags to have the canonical format: In particular,
		// the quotes may be omitted, and internal whitespace is preserved.
		if ft.Name == "_" {
			if t := fieldTag("help-summary", ft); t != "" && item.summary == nil {
				item.summary = func() string { return t }
			}
			if t := fieldTag("help-long", ft); t != "" && item.helpText == nil {
				item.helpText = func() string { return t }
			}
		}
	}
	return item, nil
}

func registerFlag(fs *flag.FlagSet, fv interface{}, name, help string) error {
	switch t := fv.(type) {
	case flag.Value:
		fs.Var(t, name, help)
	case *bool:
		fs.BoolVar(t, name, *t, help)
	case *time.Duration:
		fs.DurationVar(t, name, *t, help)
	case *float64:
		fs.Float64Var(t, name, *t, help)
	case *int64:
		fs.Int64Var(t, name, *t, help)
	case *int:
		fs.IntVar(t, name, *t, help)
	case *string:
		fs.StringVar(t, name, *t, help)
	case *uint64:
		fs.Uint64Var(t, name, *t, help)
	case *uint:
		fs.UintVar(t, name, *t, help)
	default:
		return xerrors.Errorf("type %T does not implement flag.Value", fv)
	}
	return nil
}

func fieldTag(name string, ft reflect.StructField) string {
	if t := ft.Tag.Get(name); t != "" {
		return t
	}
	s := string(ft.Tag)
	if t := strings.TrimPrefix(s, name+":"); t != s {
		return cleanString(t)
	}
	return ""
}

// cleanString removes surrounding whitespace and quotation marks from s.
func cleanString(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
}
