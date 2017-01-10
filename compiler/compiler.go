package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
)

var pySelf = py.Identifier("self")

type Module struct {
	Imports   []py.Stmt
	Classes   []*py.ClassDef
	Functions []*py.FunctionDef
	Methods   map[py.Identifier][]*py.FunctionDef
}

func newModule() *Module {
	return &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
}

func identifier(ident *ast.Ident) py.Identifier {
	return py.Identifier(ident.Name)
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

type FuncDecl struct {
	Class py.Identifier // "" if free function
	Def   *py.FunctionDef
}

func compileFuncDecl(decl *ast.FuncDecl) FuncDecl {
	var recvType py.Identifier
	pyArgs := py.Arguments{}
	if decl.Recv != nil {
		if len(decl.Recv.List) > 1 || len(decl.Recv.List[0].Names) > 1 {
			panic("multiple receivers")
		}
		field := decl.Recv.List[0]
		name := pySelf
		if len(field.Names) == 1 {
			name = identifier(field.Names[0])
		}
		pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: name})
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
	return FuncDecl{
		Class: recvType,
		Def:   &py.FunctionDef{Name: identifier(decl.Name), Args: pyArgs, Body: pyBody}}
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
	args := []py.Arg{py.Arg{Arg: pySelf}}
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
						Value: &py.Name{Id: pySelf},
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
	var targets []py.Expr
	var values []py.Expr

	// Three cases here:
	// 1. There are no values, in which case everything is zero-initialized.
	// 2. There is a value for each name.
	// 3. There is one value and it's a function returning multiple values.

	// Go                     Python
	// var x, y int           x, y = 0, 0
	// var x, y int = 1, 2    x, y = 1, 2
	// var x, y int = f()     x, y = f()

	for i, ident := range spec.Names {
		target := compileIdent(ident)

		if len(spec.Values) == 0 {
			value := nilValue(spec.Type)
			values = append(values, value)
		} else if i < len(spec.Values) {
			value := compileExpr(spec.Values[i])
			values = append(values, value)
		}

		targets = append(targets, target)
	}
	return []py.Stmt{
		&py.Assign{
			Targets: targets,
			Value:   makeTuple(values),
		},
	}
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
		funcDecl := compileFuncDecl(d)
		if funcDecl.Class != py.Identifier("") {
			module.Methods[funcDecl.Class] = append(module.Methods[funcDecl.Class], funcDecl.Def)
		} else {
			module.Functions = append(module.Functions, funcDecl.Def)
		}
	case *ast.GenDecl:
		compileGenDecl(d, module)
	default:
		panic(fmt.Sprintf("unknown Decl: %T", decl))
	}

}

func compileFile(file *ast.File, module *Module) {
	for _, decl := range file.Decls {
		compileDecl(decl, module)
	}
}

func CompilePackage(pkg *ast.Package) *py.Module {
	module := &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
	for _, file := range pkg.Files {
		compileFile(file, module)
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
