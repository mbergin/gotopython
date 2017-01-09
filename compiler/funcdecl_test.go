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
		pyFuncDecl := compileFuncDecl(funcDecl)
		if !reflect.DeepEqual(pyFuncDecl, test.python) {
			t.Errorf("want \n%s got \n%s", sp.Sdump(test.python), sp.Sdump(pyFuncDecl))
		}
	}
}
