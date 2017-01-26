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

	// Var decl shadows that of outer scope. Python only has function level scopes so vars must be renamed
	{"func f() { x := 1; {x := 2; _ = x}; _ = x }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x1")}}, Value: two},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: &py.Name{Id: py.Identifier("x1")}},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: &py.Name{Id: py.Identifier("x")}},
		},
	}}},

	// Function literals
	{"func f() { x := 1; func(y int) { _ = x; _ = y }(1) }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
			&py.FunctionDef{
				Name: py.Identifier("func"),
				Args: py.Arguments{
					Args: []py.Arg{
						{Arg: py.Identifier("y")},
					},
				},
				Body: []py.Stmt{
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: x},
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: y},
				},
			},
			&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("func")}, Args: []py.Expr{one}}},
		},
	}}},

	// Function literals create a new python scope so variables do not need to be renamed
	{"func f() { x := 1; func(x int) { _ = x }(1); _ = x }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
			&py.FunctionDef{
				Name: py.Identifier("func"),
				Args: py.Arguments{
					Args: []py.Arg{
						{Arg: py.Identifier("x")},
					},
				},
				Body: []py.Stmt{
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: x},
				},
			},
			&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("func")}, Args: []py.Expr{one}}},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: x},
		},
	}}},
	{"func f() { x := 1; func() { x := 1; _ = x }(); _ = x }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
			&py.FunctionDef{
				Name: py.Identifier("func"),
				Body: []py.Stmt{
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: x},
				},
			},
			&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("func")}}},
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: x},
		},
	}}},

	// Defer
	{"func f() { x := 1; defer ignore(x); _ = x }", FuncDecl{noClass, &py.FunctionDef{
		Name: f,
		Body: []py.Stmt{
			&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("defers")}}, Value: &py.List{}},
			&py.Try{
				Body: []py.Stmt{
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("x")}}, Value: one},
					&py.ExprStmt{&py.Call{
						Func: &py.Attribute{Value: &py.Name{Id: py.Identifier("defers")}, Attr: py.Identifier("append")},
						Args: []py.Expr{&py.Tuple{Elts: []py.Expr{&py.Name{Id: py.Identifier("ignore")}, &py.Tuple{Elts: []py.Expr{&py.Name{Id: py.Identifier("x")}}}}}},
					}},
					&py.Assign{Targets: []py.Expr{&py.Name{Id: py.Identifier("_")}}, Value: &py.Name{Id: py.Identifier("x")}},
				},
				Finalbody: []py.Stmt{
					&py.For{
						Target: &py.Tuple{Elts: []py.Expr{&py.Name{Id: "fun"}, &py.Name{Id: "args"}}},
						Iter:   &py.Call{Func: pyReversed, Args: []py.Expr{&py.Name{Id: py.Identifier("defers")}}},
						Body:   []py.Stmt{&py.ExprStmt{&py.Call{Func: &py.Name{Id: "fun"}, Args: []py.Expr{&py.Starred{Value: &py.Name{Id: "args"}}}}}},
					},
				},
			},
		},
	}}},
}

func TestFuncDecl(t *testing.T) {
	for _, test := range funcDeclTests {
		t.Run(test.golang, func(t *testing.T) {
			pkg, file, errs := buildFile(fmt.Sprintf(funcDeclPkgTemplate, test.golang))
			if errs != nil {
				t.Errorf("failed to build Go func decl %q", test.golang)
				for _, e := range errs {
					t.Error(e)
				}
				t.FailNow()
			}

			c := NewCompiler(&pkg.Info, nil)

			goFuncDecl := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			pyFuncDecl := c.compileFuncDecl(goFuncDecl)
			if !reflect.DeepEqual(pyFuncDecl, test.python) {
				t.Errorf("%q\nwant:\n%s\ngot:\n%s\n", test.golang,
					pythonCode([]py.Stmt{test.python.Def}), pythonCode([]py.Stmt{pyFuncDecl.Def}))
			}
		})
	}
}
