package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
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

type Compiler struct {
	*types.Info
	*scope
	*token.FileSet
}

func NewCompiler(typeInfo *types.Info, fileSet *token.FileSet) *Compiler {
	return &Compiler{typeInfo, newScope(), fileSet}
}

func (parent *Compiler) nestedCompiler() *Compiler {
	return &Compiler{parent.Info, parent.scope.nested(), parent.FileSet}
}

func (c *Compiler) exprCompiler() *exprCompiler {
	return &exprCompiler{Compiler: c}
}

func (c *Compiler) newModule() *Module {
	return &Module{Methods: map[py.Identifier][]*py.FunctionDef{}}
}

func (c *Compiler) err(node ast.Node, msg string, args ...interface{}) string {
	if c.FileSet != nil {
		return fmt.Sprintf("%s: %s", c.Position(node.Pos()), fmt.Sprintf(msg, args...))
	} else {
		return fmt.Sprintf(msg, args...)
	}
}

// TODO get rid of this
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
		panic(c.err(field, "unknown field type: %T", field.Type))
	}
	return c.identifier(ident)
}

type FuncDecl struct {
	Class py.Identifier // "" if free function
	Def   *py.FunctionDef
}

func (parent *Compiler) compileFunc(name py.Identifier, typ *ast.FuncType, body *ast.BlockStmt, isMethod bool, recv *ast.Ident) *py.FunctionDef {
	pyArgs := py.Arguments{}
	// Compiler with nested function scope
	c := parent.nestedCompiler()
	if isMethod {
		var recvId py.Identifier
		if recv != nil {
			recvId = c.identifier(recv)
		} else {
			recvId = c.tempID("self")
		}
		pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: recvId})
	}
	for _, param := range typ.Params.List {
		for _, name := range param.Names {
			pyArgs.Args = append(pyArgs.Args, py.Arg{Arg: c.identifier(name)})
		}
	}
	var pyBody []py.Stmt
	for _, stmt := range body.List {
		pyBody = append(pyBody, c.compileStmt(stmt)...)
	}
	if len(pyBody) == 0 {
		pyBody = []py.Stmt{&py.Pass{}}
	}
	return &py.FunctionDef{Name: name, Args: pyArgs, Body: pyBody}
}

func (c *Compiler) compileFuncDecl(decl *ast.FuncDecl) FuncDecl {
	var recvType py.Identifier
	var recv *ast.Ident
	if decl.Recv != nil {
		if len(decl.Recv.List) > 1 || len(decl.Recv.List[0].Names) > 1 {
			panic(c.err(decl, "multiple receivers"))
		}
		field := decl.Recv.List[0]
		if len(field.Names) == 1 {
			recv = field.Names[0]
		}
		recvType = c.fieldType(field)
	}
	funcDef := c.compileFunc(c.identifier(decl.Name), decl.Type, decl.Body, decl.Recv != nil, recv)
	return FuncDecl{Class: recvType, Def: funcDef}
}

func (c *Compiler) zeroValue(typ types.Type) py.Expr {
	switch t := typ.(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Signature, *types.Interface, *types.Chan:
		return pyNone
	case *types.Basic:
		switch {
		case t.Info()&types.IsString != 0:
			return &py.Str{S: "\"\""}
		case t.Info()&types.IsBoolean != 0:
			return &py.NameConstant{Value: py.False}
		case t.Info()&types.IsInteger != 0:
			return &py.Num{N: "0"}
		case t.Info()&types.IsFloat != 0:
			return &py.Num{N: "0.0"}
		default:
			panic(fmt.Sprintf("unknown basic type %#v", t))
		}
	case *types.Named:
		return &py.Call{Func: &py.Name{Id: py.Identifier(t.Obj().Name())}}
	case *types.Array:
		return &py.ListComp{
			Elt: c.zeroValue(t.Elem()),
			Generators: []py.Comprehension{
				py.Comprehension{
					Target: &py.Name{Id: py.Identifier("_")},
					Iter: &py.Call{
						Func: pyRange,
						Args: []py.Expr{&py.Num{N: strconv.FormatInt(t.Len(), 10)}},
					},
				},
			},
		}
	default:
		panic(fmt.Sprintf("unknown zero value for %T", t))
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
			dflt := c.zeroValue(c.TypeOf(name))
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

func (c *Compiler) compileInterfaceType(ident *ast.Ident, typ *ast.InterfaceType) py.Stmt {
	return nil
}

func (c *Compiler) compileTypeSpec(spec *ast.TypeSpec) py.Stmt {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		return c.compileStructType(spec.Name, t)
	case *ast.Ident:
		return &py.Assign{Targets: []py.Expr{c.compileIdent(spec.Name)}, Value: c.compileIdent(t)}
	case *ast.InterfaceType:
		return c.compileInterfaceType(spec.Name, t)
	default:
		panic(c.err(spec, "unknown TypeSpec: %T", spec.Type))
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
			c.err(s, "unknown Spec: %T", s)
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
		panic(c.err(decl, "unknown Decl: %T", decl))
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
