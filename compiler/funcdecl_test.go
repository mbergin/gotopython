package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"reflect"
	"testing"
)

// Each test compiles this code with the function declaration under test substituted for %s
const funcDeclPkgTemplate = `package main

type T struct{x, y int}
var (
	t0 = T{}
	t1 = T{}
)

type U struct{}

var (
	b0, b1 bool
	w, x, y, z int
	u0, u1 uint
	xs []int
	obj interface{}
	m map[int]int
)

func ignore(interface{}) {}
func f0() int { return 0 }
func f1(int) int { return 0 }
func f2(int, int) int { return 0 }

func g2() (int, int) { return 0, 0 }

func s(...interface{}) {}

%s
`

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
	{"func f(x T) U { return U{} }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{&py.Return{Value: &py.Call{Func: U}}},
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

	// Test that identifiers that hide builtins can be called
	{"func make(); func f() { make() }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("make")}}},
		},
	}}},
	{"func f() { true := 1; _ = true }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("true")}}, Value: one},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: &py.Name{Id: py.Identifier("true")}},
		},
	}}},
}

func TestFuncDecl(t *testing.T) {
	for _, test := range funcDeclTests {
		pkg, file, errs := buildFile(fmt.Sprintf(funcDeclPkgTemplate, test.golang))
		if errs != nil {
			t.Errorf("failed to build Go func decl %q", test.golang)
			for _, e := range errs {
				t.Error(e)
			}
			continue
		}

		c := NewCompiler(pkg.Info)

		goFuncDecl := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
		pyFuncDecl := c.compileFuncDecl(goFuncDecl)
		if !reflect.DeepEqual(pyFuncDecl, test.python) {
			t.Errorf("%q\nwant:\n%s\ngot:\n%s\n", test.golang,
				pythonCode([]py.Stmt{test.python.Def}), pythonCode([]py.Stmt{pyFuncDecl.Def}))
		}
	}
}
