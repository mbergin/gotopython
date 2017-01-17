package compiler

import (
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

var noClass py.Identifier

var funcDeclTests = []struct {
	golang string
	python FuncDecl
}{
	// Function decl
	{"func f() {}", FuncDecl{noClass, &py.FunctionDef{Name: f, Body: []py.Stmt{&py.Pass{}}}}},
	{"func f() {s(0)}", FuncDecl{noClass, &py.FunctionDef{Name: f, Body: s(0)}}},
	{"func f(x T) {s(0)}", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: s(0),
		Args: py.Arguments{
			Args: []py.Arg{py.Arg{Arg: x.Id}},
		},
	}}},
	{"func f(x T) U {s(0)}", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: s(0),
		Args: py.Arguments{
			Args: []py.Arg{py.Arg{Arg: x.Id}},
		},
	}}},
	{"func (x T) f() {s(0)}", FuncDecl{T.Id, &py.FunctionDef{
		Name: f,
		Body: s(0),
		Args: py.Arguments{
			Args: []py.Arg{
				py.Arg{Arg: x.Id},
			},
		},
	}}},
	{"func (x *T) f() {s(0)}", FuncDecl{T.Id, &py.FunctionDef{
		Name: f,
		Body: s(0),
		Args: py.Arguments{
			Args: []py.Arg{
				py.Arg{Arg: x.Id},
			},
		},
	}}},
	{"func (T) f() {s(0)}", FuncDecl{T.Id, &py.FunctionDef{
		Name: f,
		Body: s(0),
		Args: py.Arguments{
			Args: []py.Arg{
				py.Arg{Arg: pySelf},
			},
		},
	}}},

	// Return
	{"func f() { return }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Return{},
		},
	}}},
	// TODO named return values
	// {"func f() (x int) { return }", FuncDecl{noClass, &py.FunctionDef{
	// 	Name: f,
	// 	Body: []py.Stmt{
	// 		&py.Assign{Targets: []py.Expr{x}, Value: &py.Num{N: "0"}},
	// 		&py.Return{Value: x},
	// 	},
	// }}},
}

func parseFuncDecl(stmt string) (*ast.FuncDecl, error) {
	stmt = "package file\n" + stmt
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "file.go", stmt, 0)
	if err != nil {
		return nil, err
	}
	pkg := &ast.Package{
		Name:  "file",
		Files: map[string]*ast.File{"file.go": file},
	}
	if err != nil {
		return nil, err
	}
	return pkg.Files["file.go"].Decls[0].(*ast.FuncDecl), nil
}

func TestFuncDecl(t *testing.T) {
	for _, test := range funcDeclTests {
		funcDecl, err := parseFuncDecl(test.golang)
		if err != nil {
			t.Errorf("failed to parse Go stmt %q: %s", test.golang, err)
			continue
		}
		c := &Compiler{}
		pyFuncDecl := c.compileFuncDecl(funcDecl)
		if !reflect.DeepEqual(pyFuncDecl, test.python) {
			t.Errorf("want \n%s got \n%s", sp.Sdump(test.python), sp.Sdump(pyFuncDecl))
		}
	}
}
