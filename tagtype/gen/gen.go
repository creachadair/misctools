// Package gen defines a code generator for custom JSON marshaling
// and unmarshaling methods. The resulting methods encode a value of
// the type as a JSON object with two fields:
//
//    {"type": "<type-tag>", "value": <json-encoded-value>}
//
// Unmarshaling requires an object of this shape with the type tag
// corresponding to the object.
//
// The generator works for types having a method that reports the desired tag:
//
//   func (T) jsonWrapperTag() string { ... }
//   func (*T) jsonWrapperTag() string { ... }
//
// Types that do not have such a method are ignored.
package gen

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"strings"
)

const (
	tagMethodName = "jsonWrapperTag" // the name of the tag method
	wrapperType   = `struct {
   T string          ` + "`" + `json:"type"` + "`" + `
   V json.RawMessage ` + "`" + `json:"value"` + "`" + `
}`
)

// Parse parses the Go source files in the specified directory, and returns the
// resulting non-test package. It ie
func Parse(dir string) (*ast.Package, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, pkg := range pkgs {
		if !strings.HasSuffix(pkg.Name, "_test") {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("no non-test package found in %q", dir)
}

// FindTypes returns the names of all the declared top-level types T in pkg
// having a method with one of these type signatures:
//
//    func (T) jsonWrapperTag() string { ... }
//    func (*T) jsonWrapperTag() string { ... }
//
func FindTypes(pkg *ast.Package) []string {
	var types []string
	for _, file := range pkg.Files {
		// Search for tag methods. Since methods can only be declared at the top
		// level, we only need to look at top-level declarations.
		for _, decl := range file.Decls {
			name, ok := checkFuncType(decl)
			if !ok {
				continue
			}
			types = append(types, name)
		}
	}
	return types
}

// FormatSource formats raw as Go source text and writes the result to w.
// In case of an error in formatting, raw is written to w before reporting the
// error, so that the caller can debug code generation problems.
func FormatSource(w io.Writer, raw []byte) error {
	src, err := format.Source(raw)
	if err != nil {
		w.Write(raw)
		return err
	}
	_, err = w.Write(src)
	return err
}

// GenerateMarshal emits a MarshalJSON method to w for the given type name.
func GenerateMarshal(w io.Writer, name string) {
	fmt.Fprintf(w, `
// MarshalJSON implements the json.Marshaler interface for %[1]s.
// It encodes the value as a tagged wrapper object.
func (v %[1]s) MarshalJSON() ([]byte, error) {
   type shim %[1]s
   data, err := json.Marshal((*shim)(&v))
   if err != nil {
      return nil, err
   }
   return json.Marshal(%[3]s{
     T: v.%[2]s(), V: data,
   })
}
`, name, tagMethodName, wrapperType)
}

// GenerateUnmarshal emits an UnmarshalJSON method to w for the given type name.
func GenerateUnmarshal(w io.Writer, name string) {
	fmt.Fprintf(w, `
// UnmarshalJSON implements the json.Unmarshaler interface for %[1]s.
// It expects a tagged wrapper object containing the encoded value.
func (v *%[1]s) UnmarshalJSON(data []byte) error {
   var wrapper %[3]s
   if err := json.Unmarshal(data, &wrapper); err != nil {
      return err
   } else if want := v.%[2]s(); wrapper.T != want {
      return fmt.Errorf("type tag %%q does not match %%q", wrapper.T, want)
   }
   type shim %[1]s
   return json.Unmarshal(wrapper.V, (*shim)(v))
}
`, name, tagMethodName, wrapperType)
}

// checkFuncType reports whether decl is a method declaration having the form
//
//    func (x T) jsonWrapperTag() string { ... }
//
// and if so, returns the name of T.
func checkFuncType(decl ast.Decl) (string, bool) {
	fd, ok := decl.(*ast.FuncDecl)
	if !ok {
		return "", false // not a function
	} else if fd.Name.Name != tagMethodName || fd.Recv == nil {
		return "", false // not a method named jsonWrapperTag
	} else if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
		return "", false // wrong return signature
	} else if len(fd.Type.Params.List) != 0 {
		return "", false // wrong parameter signature
	}

	// Require the single return to be type string.
	rt := fd.Type.Results.List[0].Type
	if id, ok := rt.(*ast.Ident); !ok || id.Name != "string" {
		return "", false
	}

	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr: // pointer receiver
		return t.X.(*ast.Ident).Name, true
	case *ast.Ident: // non-pointer receiver
		return t.Name, true
	default:
		panic("invalid receiver syntax")
	}
}
