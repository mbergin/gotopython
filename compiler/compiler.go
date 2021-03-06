package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
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
	commentMap *ast.CommentMap
	defers     py.Expr
}

func NewCompiler(typeInfo *types.Info, fileSet *token.FileSet) *Compiler {
	return &Compiler{Info: typeInfo, scope: newScope(), FileSet: fileSet}
}

func (c Compiler) nestedCompiler() *Compiler {
	c.scope = c.scope.nested()
	return &c
}

func (c Compiler) withCommentMap(cmap *ast.CommentMap) *Compiler {
	c.commentMap = cmap
	return &c
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

func (c *Compiler) identifier(ident *ast.Ident) py.Identifier {
	return c.objID(c.ObjectOf(ident))
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

func (c *Compiler) addDefers(body *ast.BlockStmt) py.Stmt {
	hasDefer := false
	ast.Inspect(body, func(node ast.Node) bool {
		// Break early if a defer was found
		if hasDefer {
			return false
		}
		switch node.(type) {
		case *ast.DeferStmt:
			c.defers = &py.Name{Id: c.tempID("defers")}
			hasDefer = true
			return false
		case ast.Stmt:
			// Recurse into if, for, etc
			return true
		}
		// Do not recurse into expressions e.g. function literals
		return false
	})
	if hasDefer {
		return &py.Assign{
			Targets: []py.Expr{c.defers},
			Value:   &py.List{},
		}
	}
	return nil
}

func (parent *Compiler) compileFunc(name py.Identifier, typ *ast.FuncType, body *ast.BlockStmt, isMethod bool, recv *ast.Ident) *py.FunctionDef {
	pyArgs := py.Arguments{}
	// Compiler with nested function scope
	c := parent.nestedCompiler()

	var pyBody []py.Stmt

	// add an empty list of defer functions before the function body if this function uses defer
	deferInit := c.addDefers(body)

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

	for _, stmt := range body.List {
		pyBody = append(pyBody, c.compileStmt(stmt)...)
	}

	// Execute defers
	if deferInit != nil {
		fun := &py.Name{Id: c.tempID("fun")}
		args := &py.Name{Id: c.tempID("args")}
		forLoop := &py.For{
			Target: makeTuple(fun, args),
			Iter:   &py.Call{Func: pyReversed, Args: []py.Expr{c.defers}},
			Body: []py.Stmt{
				&py.ExprStmt{
					Value: &py.Call{Func: fun, Args: []py.Expr{&py.Starred{Value: args}}},
				},
			},
		}
		pyBody = []py.Stmt{
			deferInit,
			&py.Try{
				Body:      pyBody,
				Finalbody: []py.Stmt{forLoop},
			},
		}
	}

	if len(pyBody) == 0 {
		pyBody = []py.Stmt{&py.Pass{}}
	}
	return &py.FunctionDef{Name: name, Args: pyArgs, Body: pyBody}
}

func makeDocString(g *ast.CommentGroup) *py.DocString {
	text := g.Text()
	text = strings.TrimRight(text, "\n")
	return &py.DocString{Lines: strings.Split(text, "\n")}
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

	if decl.Doc != nil {
		funcDef.Body = append([]py.Stmt{makeDocString(decl.Doc)}, funcDef.Body...)
	}
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

func (c *Compiler) makeInitMethod(typ *types.Struct) *py.FunctionDef {
	nested := c.nestedCompiler()
	args := []py.Arg{py.Arg{Arg: pySelf}}
	var defaults []py.Expr
	for i := 0; i < typ.NumFields(); i++ {
		field := typ.Field(i)
		arg := py.Arg{Arg: nested.objID(field)}
		args = append(args, arg)
		dflt := nested.zeroValue(field.Type())
		defaults = append(defaults, dflt)
	}

	var body []py.Stmt
	for i := 0; i < typ.NumFields(); i++ {
		field := typ.Field(i)
		assign := &py.Assign{
			Targets: []py.Expr{
				&py.Attribute{
					Value: &py.Name{Id: pySelf},
					Attr:  nested.objID(field),
				},
			},
			Value: &py.Name{Id: nested.objID(field)},
		}
		body = append(body, assign)
	}
	initMethod := &py.FunctionDef{
		Name: py.Identifier("__init__"),
		Args: py.Arguments{Args: args, Defaults: defaults},
		Body: body,
	}
	return initMethod
}

func (c *Compiler) compileStructType(ident *ast.Ident, typ *types.Struct) *py.ClassDef {
	var body []py.Stmt

	if c.commentMap != nil {
		doc := (*c.commentMap)[ident]
		if len(doc) > 0 {
			body = append(body, makeDocString(doc[0]))
		}
	}

	if typ.NumFields() > 0 {
		body = append(body, c.makeInitMethod(typ))
	}

	if len(body) == 0 {
		body = []py.Stmt{&py.Pass{}}
	}
	return &py.ClassDef{
		Name:          c.identifier(ident),
		Bases:         nil,
		Keywords:      nil,
		Body:          body,
		DecoratorList: nil,
	}
}

func (c *Compiler) compileInterfaceType(ident *ast.Ident, typ *types.Interface) py.Stmt {
	return nil
}

func (c *Compiler) compileTypeSpec(spec *ast.TypeSpec) py.Stmt {
	switch t := c.TypeOf(spec.Type).(type) {
	case *types.Struct:
		return c.compileStructType(spec.Name, t)
	case *types.Named:
		return &py.Assign{
			Targets: []py.Expr{&py.Name{Id: c.identifier(spec.Name)}},
			Value:   &py.Name{Id: c.objID(t.Obj())},
		}
	case *types.Interface:
		return c.compileInterfaceType(spec.Name, t)
	case *types.Basic, *types.Slice:
		fields := []*types.Var{types.NewField(token.NoPos, nil, "value", t, false)}
		return c.compileStructType(spec.Name, types.NewStruct(fields, nil))
	default:
		panic(c.err(spec, "unknown TypeSpec: %T", t))
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
	cmap := ast.NewCommentMap(c.FileSet, file, file.Comments)
	c1 := c.withCommentMap(&cmap)
	for _, decl := range file.Decls {
		c1.compileDecl(decl, module)
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
