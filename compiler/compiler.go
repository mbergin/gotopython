package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
)

type Module struct {
	Imports   []py.Stmt
	Classes   []*py.ClassDef
	Functions []*py.FunctionDef
	Methods   map[py.Identifier][]*py.FunctionDef
}

func identifier(ident *ast.Ident) py.Identifier {
	return py.Identifier(ident.Name)
}

func compileCaseClauseTest(caseClause *ast.CaseClause, tag py.Expr) py.Expr {
	var tests []py.Expr
	for _, expr := range caseClause.List {
		var test py.Expr
		if tag != nil {
			test = &py.Compare{
				Left:        tag,
				Ops:         []py.CmpOp{py.Eq},
				Comparators: []py.Expr{compileExpr(expr)}}
		} else {
			test = compileExpr(expr)
		}
		tests = append(tests, test)
	}
	if len(tests) == 0 {
		return nil
	} else if len(tests) == 1 {
		return tests[0]
	}
	return &py.BoolOpExpr{Op: py.Or, Values: tests}
}

func fieldType(field *ast.Field) py.Identifier {
	var ident *ast.Ident
	switch e := field.Type.(type) {
	case *ast.StarExpr:
		ident = e.X.(*ast.Ident)
	case *ast.Ident:
		ident = e
	default:
		panic(fmt.Sprintf("unknown field type: %T", field.Type))
	}
	return identifier(ident)
}

func compileFuncDecl(decl *ast.FuncDecl) (py.Identifier, *py.FunctionDef) {
	var recvType py.Identifier
	pyArgs := py.Arguments{}
	if decl.Recv != nil {
		if len(decl.Recv.List) != 1 || len(decl.Recv.List[0].Names) != 1 {
			panic("multiple receivers")
		}
		field := decl.Recv.List[0]
		name := field.Names[0]
		pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: identifier(name)})
		recvType = fieldType(field)
	}
	for _, param := range decl.Type.Params.List {
		for _, name := range param.Names {
			pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: identifier(name)})
		}
	}
	var pyBody []py.Stmt
	for _, stmt := range decl.Body.List {
		pyBody = append(pyBody, compileStmt(stmt)...)
	}
	if len(pyBody) == 0 {
		pyBody = []py.Stmt{&py.Pass{}}
	}
	return recvType, &py.FunctionDef{Name: identifier(decl.Name), Args: pyArgs, Body: pyBody}
}

func nilValue(typ ast.Expr) py.Expr {
	switch t := typ.(type) {
	case *ast.StarExpr:
		return &py.NameConstant{Value: py.None}
	case *ast.Ident:
		switch t.Name {
		case "string":
			return &py.Str{S: "\"\""}
		case "bool":
			return &py.NameConstant{Value: py.False}
		case "int":
			return &py.Num{N: "0"}
		default:
			return &py.Call{Func: compileExpr(t)}
		}
	case *ast.SelectorExpr:
		return &py.Call{Func: compileExpr(t)}
	case *ast.ArrayType:
		return &py.List{}
	default:
		panic(fmt.Sprintf("unknown nilValue for %T", t))
	}
}

func compileStructType(ident *ast.Ident, typ *ast.StructType) *py.ClassDef {
	self := py.Identifier("self")
	args := []py.Arg{py.Arg{Arg: self}}
	var defaults []py.Expr
	for _, field := range typ.Fields.List {
		for _, name := range field.Names {
			arg := py.Arg{Arg: identifier(name)}
			args = append(args, arg)
			dflt := nilValue(field.Type)
			defaults = append(defaults, dflt)
		}
	}
	var body []py.Stmt
	for _, field := range typ.Fields.List {
		for _, name := range field.Names {
			assign := &py.Assign{
				Targets: []py.Expr{
					&py.Attribute{
						Value: &py.Name{Id: self},
						Attr:  identifier(name),
					},
				},
				Value: compileIdent(name),
			}
			body = append(body, assign)
		}
	}
	initMethod := &py.FunctionDef{
		Name: py.Identifier("__init__"),
		Args: py.Arguments{Args: args, Defaults: defaults},
		Body: body,
	}
	return &py.ClassDef{
		Name:          identifier(ident),
		Bases:         nil,
		Keywords:      nil,
		Body:          []py.Stmt{initMethod},
		DecoratorList: nil,
	}
}

func compileTypeSpec(spec *ast.TypeSpec, module *Module) {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		module.Classes = append(module.Classes, compileStructType(spec.Name, t))
	default:
		panic(fmt.Sprintf("unknown TypeSpec: %T", spec.Type))
	}
}

func compileImportSpec(spec *ast.ImportSpec, module *Module) {
	//TODO
}

func compileValueSpec(spec *ast.ValueSpec) []py.Stmt {
	var stmts []py.Stmt
	for _, ident := range spec.Names {
		assign := &py.Assign{
			Targets: []py.Expr{compileIdent(ident)},
			Value:   nilValue(spec.Type)}
		stmts = append(stmts, assign)
	}
	return stmts
}

func compileGenDecl(decl *ast.GenDecl, module *Module) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			compileTypeSpec(s, module)
		case *ast.ImportSpec:
			compileImportSpec(s, module)
		default:
			panic(fmt.Sprintf("unknown Spec: %T", s))
		}
	}
}

func compileDecl(decl ast.Decl, module *Module) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		typ, fun := compileFuncDecl(d)
		if typ != py.Identifier("") {
			module.Methods[typ] = append(module.Methods[typ], fun)
		} else {
			module.Functions = append(module.Functions, fun)
		}
	case *ast.GenDecl:
		compileGenDecl(d, module)
	default:
		panic(fmt.Sprintf("unknown Decl: %T", decl))
	}

}

func CompileFile(file *ast.File, module *Module) {
	for _, decl := range file.Decls {
		compileDecl(decl, module)
	}
}

func CompilePackage(pkg *ast.Package) *py.Module {
	module := &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
	for _, file := range pkg.Files {
		CompileFile(file, module)
	}
	pyModule := &py.Module{}
	for _, class := range module.Classes {
		for _, method := range module.Methods[class.Name] {
			class.Body = append(class.Body, method)
		}
		pyModule.Body = append(pyModule.Body, class)
	}
	for _, fun := range module.Functions {
		pyModule.Body = append(pyModule.Body, fun)
	}
	return pyModule
}
