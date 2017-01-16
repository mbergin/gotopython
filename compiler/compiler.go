package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
)

var pySelf = py.Identifier("self")

type Module struct {
	Imports   []py.Stmt
	Values    []py.Stmt
	Classes   []*py.ClassDef
	Types     []py.Stmt
	Functions []*py.FunctionDef
	Methods   map[py.Identifier][]*py.FunctionDef
}

type Compiler struct{}

func (c *Compiler) newModule() *Module {
	return &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
}

func (c *Compiler) identifier(ident *ast.Ident) py.Identifier {
	return py.Identifier(ident.Name)
}

func (c *Compiler) fieldType(field *ast.Field) py.Identifier {
	var ident *ast.Ident
	switch e := field.Type.(type) {
	case *ast.StarExpr:
		ident = e.X.(*ast.Ident)
	case *ast.Ident:
		ident = e
	default:
		panic(fmt.Sprintf("unknown field type: %T", field.Type))
	}
	return c.identifier(ident)
}

type FuncDecl struct {
	Class py.Identifier // "" if free function
	Def   *py.FunctionDef
}

func (c *Compiler) compileFuncDecl(decl *ast.FuncDecl) FuncDecl {
	var recvType py.Identifier
	pyArgs := py.Arguments{}
	if decl.Recv != nil {
		if len(decl.Recv.List) > 1 || len(decl.Recv.List[0].Names) > 1 {
			panic("multiple receivers")
		}
		field := decl.Recv.List[0]
		name := pySelf
		if len(field.Names) == 1 {
			name = c.identifier(field.Names[0])
		}
		pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: name})
		recvType = c.fieldType(field)
	}
	for _, param := range decl.Type.Params.List {
		for _, name := range param.Names {
			pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: c.identifier(name)})
		}
	}
	var pyBody []py.Stmt
	for _, stmt := range decl.Body.List {
		pyBody = append(pyBody, c.compileStmt(stmt)...)
	}
	if len(pyBody) == 0 {
		pyBody = []py.Stmt{&py.Pass{}}
	}
	return FuncDecl{
		Class: recvType,
		Def:   &py.FunctionDef{Name: c.identifier(decl.Name), Args: pyArgs, Body: pyBody}}
}

func (c *Compiler) nilValue(typ ast.Expr) py.Expr {
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
			return &py.Call{Func: c.compileExpr(t)}
		}
	case *ast.SelectorExpr:
		return &py.Call{Func: c.compileExpr(t)}
	case *ast.ArrayType:
		return &py.List{}
	default:
		panic(fmt.Sprintf("unknown nilValue for %T", t))
	}
}

func (c *Compiler) compileStructType(ident *ast.Ident, typ *ast.StructType) *py.ClassDef {
	if len(typ.Fields.List) == 0 {
		return &py.ClassDef{
			Name:          c.identifier(ident),
			Bases:         nil,
			Keywords:      nil,
			Body:          []py.Stmt{&py.Pass{}},
			DecoratorList: nil,
		}
	}
	args := []py.Arg{py.Arg{Arg: pySelf}}
	var defaults []py.Expr
	for _, field := range typ.Fields.List {
		for _, name := range field.Names {
			arg := py.Arg{Arg: c.identifier(name)}
			args = append(args, arg)
			dflt := c.nilValue(field.Type)
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
						Attr:  c.identifier(name),
					},
				},
				Value: c.compileIdent(name),
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
		Name:          c.identifier(ident),
		Bases:         nil,
		Keywords:      nil,
		Body:          []py.Stmt{initMethod},
		DecoratorList: nil,
	}
}

func (c *Compiler) compileTypeSpec(spec *ast.TypeSpec) py.Stmt {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		return c.compileStructType(spec.Name, t)
	case *ast.Ident:
		return &py.Assign{Targets: []py.Expr{c.compileIdent(spec.Name)}, Value: c.compileIdent(t)}
	default:
		panic(fmt.Sprintf("unknown TypeSpec: %T", spec.Type))
	}
}

func (c *Compiler) compileImportSpec(spec *ast.ImportSpec, module *Module) {
	//TODO
}

func (c *Compiler) compileGenDecl(decl *ast.GenDecl, module *Module) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			compiled := c.compileTypeSpec(s)
			if classDef, ok := compiled.(*py.ClassDef); ok {
				module.Classes = append(module.Classes, classDef)
			} else {
				module.Types = append(module.Types, compiled)
			}
		case *ast.ImportSpec:
			c.compileImportSpec(s, module)
		case *ast.ValueSpec:
			module.Values = append(module.Values, c.compileValueSpec(s)...)
		default:
			panic(fmt.Sprintf("unknown Spec: %T", s))
		}
	}
}

func (c *Compiler) compileDecl(decl ast.Decl, module *Module) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		funcDecl := c.compileFuncDecl(d)
		if funcDecl.Class != py.Identifier("") {
			module.Methods[funcDecl.Class] = append(module.Methods[funcDecl.Class], funcDecl.Def)
		} else {
			module.Functions = append(module.Functions, funcDecl.Def)
		}
	case *ast.GenDecl:
		c.compileGenDecl(d, module)
	default:
		panic(fmt.Sprintf("unknown Decl: %T", decl))
	}

}

func (c *Compiler) compileFile(file *ast.File, module *Module) {
	for _, decl := range file.Decls {
		c.compileDecl(decl, module)
	}
}

func (c *Compiler) CompileFiles(files []*ast.File) *py.Module {
	module := &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
	for _, file := range files {
		c.compileFile(file, module)
	}
	pyModule := &py.Module{}
	pyModule.Body = append(pyModule.Body, module.Values...)
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

func (c *Compiler) CompilePackage(pkg *ast.Package) *py.Module {
	var files []*ast.File
	for _, file := range pkg.Files {
		files = append(files, file)
	}
	return c.CompileFiles(files)
}
