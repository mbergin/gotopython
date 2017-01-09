package compiler

import (
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

// Placeholders for statement blocks
var s = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("s")}}}
var t = []py.Stmt{&py.ExprStmt{Value: &py.Name{Id: py.Identifier("t")}}}

var one = &py.Num{N: "1"}

var stmtTests = []struct {
	golang string
	python []py.Stmt
}{
	{"x", []py.Stmt{&py.ExprStmt{Value: x}}},

	{"x++", []py.Stmt{&py.AugAssign{Target: x, Op: py.Add, Value: one}}},
	{"x--", []py.Stmt{&py.AugAssign{Target: x, Op: py.Sub, Value: one}}},

	{"if x {s}", []py.Stmt{&py.If{Test: x, Body: s}}},
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
			t.Errorf("want \n%s got \n%s", sp.Sdump(test.python), sp.Sdump(pyStmt))
		}
	}
}
