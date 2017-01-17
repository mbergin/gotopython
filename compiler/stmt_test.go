package compiler

import (
	"bytes"
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"reflect"
	"strconv"
	"testing"
)

// Each test compiles this code with the expression under test substituted for %s
const stmtPkgTemplate = `package main

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

func main() {
	%s
}
`

const f = py.Identifier("f")

// Name of temp variable used to store evaluated switch tag
var tag = &py.Name{Id: py.Identifier("tag")}

// Placeholders for statement blocks
var (
	ignore = &py.Name{Id: py.Identifier("ignore")}
	g2     = &py.Name{Id: py.Identifier("g2")}
)

func s(is ...interface{}) []py.Stmt {
	var args []py.Expr
	for _, i := range is {
		switch i := i.(type) {
		case int:
			args = append(args, &py.Num{N: strconv.Itoa(i)})
		case py.Expr:
			args = append(args, i)
		default:
			panic(fmt.Sprintf("%T", i))
		}
	}
	return []py.Stmt{&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("s")}, Args: args}}}
}

var (
	zero = &py.Num{N: "0"}
	one  = &py.Num{N: "1"}
	two  = &py.Num{N: "2"}
)

var stmtTests = []struct {
	golang string
	python []py.Stmt
}{
	// Empty statement
	{";", []py.Stmt{}},

	// Expression statement
	{"ignore(x)", []py.Stmt{&py.ExprStmt{Value: &py.Call{Func: ignore, Args: []py.Expr{x}}}}},

	// IncDec statements
	{"x++", []py.Stmt{&py.AugAssign{Target: x, Op: py.Add, Value: one}}},
	{"x--", []py.Stmt{&py.AugAssign{Target: x, Op: py.Sub, Value: one}}},

	// Assignments
	{"x = y", []py.Stmt{&py.Assign{Targets: []py.Expr{x}, Value: y}}},
	{"x, y = g2()", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   &py.Call{Func: g2},
	}}},
	{"x, y = y, x", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   &py.Tuple{Elts: []py.Expr{y, x}},
	}}},

	// Short variable declarations
	{"x := y; _ = x", []py.Stmt{&py.Assign{Targets: []py.Expr{x}, Value: y}}},
	{"x, y := g2(); _, _ = x, y", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   &py.Call{Func: g2},
	}}},
	{"x, y := y, x; _, _ = x, y", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   &py.Tuple{Elts: []py.Expr{y, x}},
	}}},

	// Augmented assignments
	{"x +=  y", []py.Stmt{&py.AugAssign{Op: py.Add, Target: x, Value: y}}},
	{"x -=  y", []py.Stmt{&py.AugAssign{Op: py.Sub, Target: x, Value: y}}},
	{"x |=  y", []py.Stmt{&py.AugAssign{Op: py.BitOr, Target: x, Value: y}}},
	{"x ^=  y", []py.Stmt{&py.AugAssign{Op: py.BitXor, Target: x, Value: y}}},
	{"x *=  y", []py.Stmt{&py.AugAssign{Op: py.Mult, Target: x, Value: y}}},
	{"x /=  y", []py.Stmt{&py.AugAssign{Op: py.FloorDiv, Target: x, Value: y}}}, // TODO py.Div for floats
	{"x %=  y", []py.Stmt{&py.AugAssign{Op: py.Mod, Target: x, Value: y}}},
	{"x <<= u0", []py.Stmt{&py.AugAssign{Op: py.LShift, Target: x, Value: u0}}},
	{"x >>= u0", []py.Stmt{&py.AugAssign{Op: py.RShift, Target: x, Value: u0}}},
	{"x &=  y", []py.Stmt{&py.AugAssign{Op: py.BitAnd, Target: x, Value: y}}},
	{"x &^= y", []py.Stmt{&py.AugAssign{Op: py.BitAnd, Target: x, Value: &py.UnaryOpExpr{Op: py.Invert, Operand: y}}}},

	// Branch statements
	{"for { break }", []py.Stmt{&py.While{Test: pyTrue, Body: []py.Stmt{&py.Break{}}}}},
	{"for { continue }", []py.Stmt{&py.While{Test: pyTrue, Body: []py.Stmt{&py.Continue{}}}}},

	// If statements
	{"if b0 {s(0)}", []py.Stmt{&py.If{Test: b0, Body: s(0)}}},
	{"if s(0); b0 {s(1)}", append(s(0), &py.If{Test: b0, Body: s(1)})},
	{"if b0 {s(0)} else {s(1)}", []py.Stmt{&py.If{Test: b0, Body: s(0), Orelse: s(1)}}},
	{"if b0 {s(0)} else if b1 {s(1)}", []py.Stmt{&py.If{
		Test:   b0,
		Body:   s(0),
		Orelse: []py.Stmt{&py.If{Test: b1, Body: s(1)}},
	}}},
	{"if b0 {s(0)} else if s(1); b1 {s(2)}", []py.Stmt{&py.If{
		Test:   b0,
		Body:   s(0),
		Orelse: append(s(1), &py.If{Test: b1, Body: s(2)}),
	}}},

	// Range for
	{"for x := range xs {s(x)}", []py.Stmt{
		// for x in range(len(y)): s
		&py.For{
			Target: x,
			Iter: &py.Call{
				Func: pyRange,
				Args: []py.Expr{&py.Call{Func: pyLen, Args: []py.Expr{xs}}},
			},
			Body: s(x),
		},
	}},
	{"for x, y := range xs {s(x,y)}", []py.Stmt{
		// for x, y in enumerate(z): s
		&py.For{
			Target: &py.Tuple{
				Elts: []py.Expr{x, y},
			},
			Iter: &py.Call{
				Func: pyEnumerate,
				Args: []py.Expr{xs},
			},
			Body: s(x, y),
		},
	}},
	{"for _, x := range xs {s(x)}", []py.Stmt{
		// for x in y: s
		&py.For{
			Target: x,
			Iter:   xs,
			Body:   s(x),
		},
	}},

	// For statement
	{"for {s(0)}", []py.Stmt{
		&py.While{
			Test: pyTrue,
			Body: s(0),
		},
	}},
	{"for b0 {s(x)}", []py.Stmt{
		&py.While{
			Test: b0,
			Body: s(x),
		},
	}},
	{"for s(0); b0; s(1) {s(2)}",
		append(s(0),
			&py.While{
				Test: b0,
				Body: append(s(2), s(1)...),
			}),
	},

	// Var declaration statements
	{"var x int; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   zero,
		},
	}},
	{"var x *int; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyNone,
		},
	}},
	{"var x string; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyEmptyString,
		},
	}},
	{"var x bool; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyFalse,
		},
	}},
	{"var x T; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   &py.Call{Func: T},
		},
	}},
	{"var x []T; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyNone,
		},
	}},
	{"var x, y int; _, _ = x, y", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value: &py.Tuple{
				Elts: []py.Expr{zero, zero},
			},
		},
	}},
	{"var x int = 1; _ = x", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   one,
		},
	}},
	{"var x, y int = 1, 2; _, _ = x, y", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value: &py.Tuple{
				Elts: []py.Expr{one, two},
			},
		},
	}},
	{"var x, y int = g2(); _, _ = x, y", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value:   &py.Call{Func: g2},
		},
	}},

	// Const declarations
	{"const x, y = 1, 2", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value: &py.Tuple{
				Elts: []py.Expr{one, two},
			},
		},
	}},
	{"const (x = 1; z = 2)", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   one,
		},
		&py.Assign{
			Targets: []py.Expr{z},
			Value:   two,
		},
	}},

	// Type declarations
	{"type T U", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{T},
			Value:   U,
		},
	}},
	//{"type T interface{}", []py.Stmt{}},
	//{"type T string", []py.Stmt{}},
	//{"type T int", []py.Stmt{}},
	//{"type T bool", []py.Stmt{}},
	//{"type T []U", []py.Stmt{}},
	//{"type T map[U]V", []py.Stmt{}},
	{"type T struct {}", []py.Stmt{
		&py.ClassDef{
			Name: T.Id,
			Body: []py.Stmt{&py.Pass{}},
		},
	}},
	{"type T struct { x U }", []py.Stmt{
		&py.ClassDef{
			Name: T.Id,
			Body: []py.Stmt{&py.FunctionDef{
				Name: py.Identifier("__init__"),
				Args: py.Arguments{
					Args: []py.Arg{
						py.Arg{Arg: pySelf},
						py.Arg{Arg: x.Id},
					},
					Defaults: []py.Expr{
						&py.Call{Func: U},
					},
				},
				Body: []py.Stmt{
					&py.Assign{
						Targets: []py.Expr{
							&py.Attribute{
								Value: &py.Name{Id: pySelf},
								Attr:  x.Id,
							},
						},
						Value: x,
					},
				},
			}},
		},
	}},

	// Switch statements
	{"switch {}", nil},
	{"switch x {}", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   x,
		},
	}},
	{"switch s(0); x { case y: s(1) }", []py.Stmt{
		s(0)[0],
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   x,
		},
		&py.If{
			Test: &py.Compare{Left: tag, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Eq}},
			Body: s(1),
		},
	}},
	{"switch x { case y, z: s(0); default: s(1); case w: s(2) }", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   x,
		},
		&py.If{
			Test: &py.BoolOpExpr{
				Op: py.Or,
				Values: []py.Expr{
					&py.Compare{Left: tag, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Eq}},
					&py.Compare{Left: tag, Comparators: []py.Expr{z}, Ops: []py.CmpOp{py.Eq}},
				},
			},
			Body: s(0),
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: tag, Comparators: []py.Expr{w}, Ops: []py.CmpOp{py.Eq}},
					Body:   s(2),
					Orelse: s(1),
				},
			},
		},
	}},
	{"switch { default: s(0); case x>0: s(1); case y<0: s(2) }", []py.Stmt{
		&py.If{
			Test: &py.Compare{Left: x, Comparators: []py.Expr{zero}, Ops: []py.CmpOp{py.Gt}},
			Body: s(1),
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: y, Comparators: []py.Expr{zero}, Ops: []py.CmpOp{py.Lt}},
					Body:   s(2),
					Orelse: s(0),
				},
			},
		},
	}},

	// Type switch
	{"switch s(0); obj.(type) { default: s(1); case T: s(2); case U: s(3)}", []py.Stmt{
		s(0)[0],
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   &py.Call{Func: pyType, Args: []py.Expr{obj}},
		},
		&py.If{
			Test: &py.Compare{Left: tag, Comparators: []py.Expr{T}, Ops: []py.CmpOp{py.Eq}},
			Body: s(2),
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: tag, Comparators: []py.Expr{U}, Ops: []py.CmpOp{py.Eq}},
					Body:   s(3),
					Orelse: s(1),
				},
			},
		},
	}},
	{"switch s(0); y := obj.(type) { default: s(1, y); case T: s(2); case U: s(3)}", []py.Stmt{
		s(0)[0],
		&py.Assign{
			Targets: []py.Expr{y},
			Value:   &py.Call{Func: pyType, Args: []py.Expr{obj}},
		},
		&py.If{
			Test: &py.Compare{Left: y, Comparators: []py.Expr{T}, Ops: []py.CmpOp{py.Eq}},
			Body: s(2),
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: y, Comparators: []py.Expr{U}, Ops: []py.CmpOp{py.Eq}},
					Body:   s(3),
					Orelse: s(1, y),
				},
			},
		},
	}},

	// Builtin functions
	{"delete(m, y)", []py.Stmt{
		&py.Try{
			Body: []py.Stmt{
				&py.Delete{Targets: []py.Expr{&py.Subscript{Value: m, Slice: &py.Index{Value: y}}}},
			},
			Handlers: []py.ExceptHandler{
				{Typ: &py.Name{Id: py.Identifier("KeyError")},
					Body: []py.Stmt{&py.Pass{}}},
			},
		},
	}},
}

func pythonCode(stmts []py.Stmt) string {
	var buf bytes.Buffer
	writer := py.NewWriter(&buf)
	writer.WriteModule(&py.Module{Body: stmts})
	return buf.String()
}

func TestStmt(t *testing.T) {
	for _, test := range stmtTests {
		pkg, fileScope, errs := buildFile(fmt.Sprintf(stmtPkgTemplate, test.golang))
		if errs != nil {
			t.Errorf("failed to build Go stmt %q", test.golang)
			for _, e := range errs {
				t.Error(e)
			}
			t.FailNow()
			continue
		}

		c := NewCompiler(pkg.Info)
		goStmt := fileScope.Lookup("main").Decl.(*ast.FuncDecl).Body.List[0]
		pyStmts := c.compileStmt(goStmt)
		if !reflect.DeepEqual(pyStmts, test.python) {
			t.Errorf("%q\nwant:\n%s\ngot:\n%s\n", test.golang, pythonCode(test.python), pythonCode(pyStmts))
		}
	}
}
