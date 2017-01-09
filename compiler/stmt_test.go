package compiler

import (
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

const f = py.Identifier("f")

// Placeholders for statement blocks
var (
	s = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("s")}}}
	t = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("t")}}}
	u = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("u")}}}
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

	// Declaration statements
	{"var x int", []py.Stmt{
		&py.Assign{
			Targets: []py.Expr{x},
			Value:   zero,
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

func TestStmt(t *testing.T) {

	for _, test := range stmtTests {
		goStmt, err := parseStmt(test.golang)
		if err != nil {
			t.Errorf("failed to parse Go stmt %q: %s", test.golang, err)
			continue
		}
		pyStmt := compileStmt(goStmt)
		if !reflect.DeepEqual(pyStmt, test.python) {
			t.Errorf("%q\nwant \n%s got \n%s", test.golang, sp.Sdump(test.python), sp.Sdump(pyStmt))
		}
	}
}
