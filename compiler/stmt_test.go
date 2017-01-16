package compiler

import (
	"bytes"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

const f = py.Identifier("f")

// Name of temp variable used to store evaluated switch tag
var tag = &py.Name{Id: py.Identifier("tag")}

// Placeholders for statement blocks
var (
	s = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("s")}}}
	t = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("t")}}}
	u = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("u")}}}
	v = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("v")}}}
)

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
	{"x", []py.Stmt{&py.ExprStmt{Value: x}}},

	// IncDec statements
	{"x++", []py.Stmt{&py.AugAssign{Target: x, Op: py.Add, Value: one}}},
	{"x--", []py.Stmt{&py.AugAssign{Target: x, Op: py.Sub, Value: one}}},

	// Assignments
	{"x = y", []py.Stmt{&py.Assign{Targets: []py.Expr{x}, Value: y}}},
	{"x = y, z", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x},
		Value:   &py.Tuple{Elts: []py.Expr{y, z}},
	}}},
	{"x, y = z", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   z,
	}}},
	{"x, y = y, x", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   &py.Tuple{Elts: []py.Expr{y, x}},
	}}},

	// Short variable declarations
	{"x := y", []py.Stmt{&py.Assign{Targets: []py.Expr{x}, Value: y}}},
	{"x := y, z", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x},
		Value:   &py.Tuple{Elts: []py.Expr{y, z}},
	}}},
	{"x, y := z", []py.Stmt{&py.Assign{
		Targets: []py.Expr{x, y},
		Value:   z,
	}}},
	{"x, y := y, x", []py.Stmt{&py.Assign{
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
	{"x <<= y", []py.Stmt{&py.AugAssign{Op: py.LShift, Target: x, Value: y}}},
	{"x >>= y", []py.Stmt{&py.AugAssign{Op: py.RShift, Target: x, Value: y}}},
	{"x &=  y", []py.Stmt{&py.AugAssign{Op: py.BitAnd, Target: x, Value: y}}},
	{"x &^= y", []py.Stmt{&py.AugAssign{Op: py.BitAnd, Target: x, Value: &py.UnaryOpExpr{Op: py.Invert, Operand: y}}}},

	// Branch statements
	{"break", []py.Stmt{&py.Break{}}},
	{"continue", []py.Stmt{&py.Continue{}}},

	// If statements
	{"if x {s}", []py.Stmt{&py.If{Test: x, Body: s}}},
	{"if s; x {t}", append(s, &py.If{Test: x, Body: t})},
	{"if x {s} else {t}", []py.Stmt{&py.If{Test: x, Body: s, Orelse: t}}},
	{"if x {s} else if y {t}", []py.Stmt{&py.If{
		Test:   x,
		Body:   s,
		Orelse: []py.Stmt{&py.If{Test: y, Body: t}},
	}}},
	{"if x {s} else if t; y {u}", []py.Stmt{&py.If{
		Test:   x,
		Body:   s,
		Orelse: append(t, &py.If{Test: y, Body: u}),
	}}},

	// Range for
	{"for x := range y {s}", []py.Stmt{
		// for x in range(len(y)): s
		&py.For{
			Target: x,
			Iter: &py.Call{
				Func: pyRange,
				Args: []py.Expr{&py.Call{Func: pyLen, Args: []py.Expr{y}}},
			},
			Body: s,
		},
	}},
	{"for x, y := range z {s}", []py.Stmt{
		// for x, y in enumerate(z): s
		&py.For{
			Target: &py.Tuple{
				Elts: []py.Expr{x, y},
			},
			Iter: &py.Call{
				Func: pyEnumerate,
				Args: []py.Expr{z},
			},
			Body: s,
		},
	}},
	{"for _, x := range y {s}", []py.Stmt{
		// for x in y: s
		&py.For{
			Target: x,
			Iter:   y,
			Body:   s,
		},
	}},

	// For statement
	{"for {s}", []py.Stmt{
		&py.While{
			Test: pyTrue,
			Body: s,
		},
	}},
	{"for x {s}", []py.Stmt{
		&py.While{
			Test: x,
			Body: s,
		},
	}},
	{"for s; y; t {u}",
		append(s,
			&py.While{
				Test: y,
				Body: append(u, t...),
			}),
	},

	// Var declaration statements
	{"var x int", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   zero,
		},
	}},
	{"var x *int", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyNone,
		},
	}},
	{"var x string", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyEmptyString,
		},
	}},
	{"var x bool", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   pyFalse,
		},
	}},
	{"var x T", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   &py.Call{Func: T},
		},
	}},
	{"var x []T", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   &py.List{},
		},
	}},
	{"var x, y int", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value: &py.Tuple{
				Elts: []py.Expr{zero, zero},
			},
		},
	}},
	{"var x int = 1", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   one,
		},
	}},
	{"var x, y int = 1, 2", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value: &py.Tuple{
				Elts: []py.Expr{one, two},
			},
		},
	}},
	{"var x, y int = z()", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x, y},
			Value:   &py.Call{Func: z},
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
	{"const (x = y; z = w)", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   y,
		},
		&py.Assign{
			Targets: []py.Expr{z},
			Value:   w,
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
	{"switch s; x { case y: t }", []py.Stmt{
		s[0],
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   x,
		},
		&py.If{
			Test: &py.Compare{Left: tag, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Eq}},
			Body: t,
		},
	}},
	{"switch x { case y, z: s; default: u; case w: t }", []py.Stmt{
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
			Body: s,
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: tag, Comparators: []py.Expr{w}, Ops: []py.CmpOp{py.Eq}},
					Body:   t,
					Orelse: u,
				},
			},
		},
	}},
	{"switch { default: u; case x>0: s; case y<0: t }", []py.Stmt{
		&py.If{
			Test: &py.Compare{Left: x, Comparators: []py.Expr{zero}, Ops: []py.CmpOp{py.Gt}},
			Body: s,
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: y, Comparators: []py.Expr{zero}, Ops: []py.CmpOp{py.Lt}},
					Body:   t,
					Orelse: u,
				},
			},
		},
	}},

	// Type switch
	{"switch s; x.(type) { default: t; case T: u; case U: v}", []py.Stmt{
		s[0],
		&py.Assign{
			Targets: []py.Expr{tag},
			Value:   &py.Call{Func: pyType, Args: []py.Expr{x}},
		},
		&py.If{
			Test: &py.Compare{Left: tag, Comparators: []py.Expr{T}, Ops: []py.CmpOp{py.Eq}},
			Body: u,
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: tag, Comparators: []py.Expr{U}, Ops: []py.CmpOp{py.Eq}},
					Body:   v,
					Orelse: t,
				},
			},
		},
	}},
	{"switch s; y := x.(type) { default: t; case T: u; case U: v}", []py.Stmt{
		s[0],
		&py.Assign{
			Targets: []py.Expr{y},
			Value:   &py.Call{Func: pyType, Args: []py.Expr{x}},
		},
		&py.If{
			Test: &py.Compare{Left: y, Comparators: []py.Expr{T}, Ops: []py.CmpOp{py.Eq}},
			Body: u,
			Orelse: []py.Stmt{
				&py.If{
					Test:   &py.Compare{Left: y, Comparators: []py.Expr{U}, Ops: []py.CmpOp{py.Eq}},
					Body:   v,
					Orelse: t,
				},
			},
		},
	}},

	// Builtin functions
	{"delete(x, y)", []py.Stmt{
		&py.Try{
			Body: []py.Stmt{
				&py.Delete{Targets: []py.Expr{&py.Subscript{Value: x, Slice: &py.Index{Value: y}}}},
			},
			Handlers: []py.ExceptHandler{
				{Typ: &py.Name{Id: py.Identifier("KeyError")},
					Body: []py.Stmt{&py.Pass{}}},
			},
		},
	}},
}

func parseStmt(stmt string) (ast.Stmt, error) {
	stmt = "package file\nfunc f() {\n" + stmt + "\n}\n"
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
	return pkg.Files["file.go"].Decls[0].(*ast.FuncDecl).Body.List[0], nil
}

func pythonCode(stmts []py.Stmt) string {
	var buf bytes.Buffer
	writer := py.NewWriter(&buf)
	writer.WriteModule(&py.Module{Body: stmts})
	return buf.String()
}

func TestStmt(t *testing.T) {

	for _, test := range stmtTests {
		goStmt, err := parseStmt(test.golang)
		if err != nil {
			t.Errorf("failed to parse Go stmt %q: %s", test.golang, err)
			continue
		}
		c := &Compiler{}
		pyStmt := c.compileStmt(goStmt)
		if !reflect.DeepEqual(pyStmt, test.python) {
			t.Errorf("%q\nwant:\n%s\ngot:\n%s\n", test.golang, pythonCode(test.python), pythonCode(pyStmt))
		}
	}
}
