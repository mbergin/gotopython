package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

type exprCompiler struct {
	*Compiler
	stmts []py.Stmt
}

func (c *Compiler) compileIdent(ident *ast.Ident) py.Expr {
	if c.isBlank(ident) {
		return &py.Name{Id: py.Identifier("_")}
	}
	obj := c.ObjectOf(ident)
	if obj == nil {
		panic(fmt.Sprintf("Ident has no object: %#v", ident))
	}
	switch obj {
	case builtin.true:
		return pyTrue
	case builtin.false:
		return pyFalse
	case builtin.nil:
		return pyNone
	default:
		return &py.Name{Id: c.id(obj)}
	}
}

func comparator(t token.Token) (py.CmpOp, bool) {
	switch t {
	case token.EQL:
		return py.Eq, true
	case token.LSS:
		return py.Lt, true
	case token.GTR:
		return py.Gt, true
	case token.NEQ:
		return py.NotEq, true
	case token.LEQ:
		return py.LtE, true
	case token.GEQ:
		return py.GtE, true
	}
	return py.CmpOp(0), false
}

func binOp(t token.Token) (py.Operator, bool) {
	switch t {
	case token.ADD:
		return py.Add, true
	case token.SUB:
		return py.Sub, true
	case token.MUL:
		return py.Mult, true
	case token.QUO:
		return py.FloorDiv, true
	case token.REM:
		return py.Mod, true
	case token.AND:
		return py.BitAnd, true
	case token.OR:
		return py.BitOr, true
	case token.XOR:
		return py.BitXor, true
	case token.SHL:
		return py.LShift, true
	case token.SHR:
		return py.RShift, true
		//case token.AND_NOT: // no &^ in python so special-cased
	}
	return py.Operator(0), false
}

func boolOp(t token.Token) (py.BoolOp, bool) {
	switch t {
	case token.LAND:
		return py.And, true
	case token.LOR:
		return py.Or, true
	}
	return py.BoolOp(0), false
}

func (c *exprCompiler) compileBinaryExpr(expr *ast.BinaryExpr) py.Expr {
	if pyCmp, ok := comparator(expr.Op); ok {
		return &py.Compare{
			Left:        c.compileExpr(expr.X),
			Ops:         []py.CmpOp{pyCmp},
			Comparators: []py.Expr{c.compileExpr(expr.Y)}}
	}
	if pyOp, ok := binOp(expr.Op); ok {
		return &py.BinOp{Left: c.compileExpr(expr.X),
			Right: c.compileExpr(expr.Y),
			Op:    pyOp}
	}
	if pyBoolOp, ok := boolOp(expr.Op); ok {
		return &py.BoolOpExpr{
			Values: []py.Expr{c.compileExpr(expr.X), c.compileExpr(expr.Y)},
			Op:     pyBoolOp}
	}
	if expr.Op == token.AND_NOT {
		return &py.BinOp{Left: c.compileExpr(expr.X),
			Right: &py.UnaryOpExpr{Op: py.Invert, Operand: c.compileExpr(expr.Y)},
			Op:    py.BitAnd}
	}
	panic(c.err(expr, "unknown BinaryExpr Op: %v", expr.Op))
}

func (c *exprCompiler) compileBasicLit(expr *ast.BasicLit) py.Expr {
	switch expr.Kind {
	case token.INT, token.FLOAT:
		return &py.Num{N: expr.Value}
	case token.CHAR:
		return &py.Str{S: expr.Value}
	case token.STRING:
		return &py.Str{S: expr.Value}
	case token.IMAG:
		return &py.Num{N: strings.Replace(expr.Value, "i", "j", 1)}
	}
	panic(c.err(expr, "unknown BasicLit kind: %v", expr.Kind))
}

func (c *exprCompiler) compileUnaryExpr(expr *ast.UnaryExpr) py.Expr {
	switch expr.Op {
	case token.NOT:
		return &py.UnaryOpExpr{Op: py.Not, Operand: c.compileExpr(expr.X)}
	case token.AND: // address of
		return c.compileExpr(expr.X)
	case token.ADD:
		return &py.UnaryOpExpr{Op: py.UAdd, Operand: c.compileExpr(expr.X)}
	case token.SUB:
		return &py.UnaryOpExpr{Op: py.USub, Operand: c.compileExpr(expr.X)}
	case token.XOR:
		return &py.UnaryOpExpr{Op: py.Invert, Operand: c.compileExpr(expr.X)}
	}
	panic(c.err(expr, "unknown UnaryExpr: %v", expr.Op))
}

func (c *exprCompiler) compileCompositeLit(expr *ast.CompositeLit) py.Expr {
	switch typ := c.TypeOf(expr).(type) {
	case *types.Named:
		var args []py.Expr
		var keywords []py.Keyword
		if len(expr.Elts) > 0 {
			if _, ok := expr.Elts[0].(*ast.KeyValueExpr); ok {
				for _, elt := range expr.Elts {
					kv := elt.(*ast.KeyValueExpr)
					id := c.identifier(kv.Key.(*ast.Ident))
					keyword := py.Keyword{
						Arg:   &id,
						Value: c.compileExpr(kv.Value)}
					keywords = append(keywords, keyword)
				}
			} else {
				args = make([]py.Expr, len(expr.Elts))
				for i, elt := range expr.Elts {
					args[i] = c.compileExpr(elt)
				}
			}
		}
		return &py.Call{
			Func:     &py.Name{Id: c.id(typ.Obj())},
			Args:     args,
			Keywords: keywords,
		}
	case *types.Array:
		elts := make([]py.Expr, len(expr.Elts))
		for i, elt := range expr.Elts {
			elts[i] = c.compileExpr(elt)
		}
		return &py.List{Elts: elts}
	case *types.Slice:
		elts := make([]py.Expr, len(expr.Elts))
		for i, elt := range expr.Elts {
			elts[i] = c.compileExpr(elt)
		}
		return &py.List{Elts: elts}
	case *types.Map:
		keys := make([]py.Expr, len(expr.Elts))
		values := make([]py.Expr, len(expr.Elts))
		for i, elt := range expr.Elts {
			kv := elt.(*ast.KeyValueExpr)
			keys[i] = c.compileExpr(kv.Key)
			values[i] = c.compileExpr(kv.Value)
		}
		return &py.Dict{Keys: keys, Values: values}
	default:
		panic(c.err(expr, "Unknown composite literal type: %T", typ))
	}
}

func (c *exprCompiler) compileSelectorExpr(expr *ast.SelectorExpr) py.Expr {
	return &py.Attribute{
		Value: c.compileExpr(expr.X),
		Attr:  c.identifier(expr.Sel),
	}
}

func isString(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsString != 0
}

var builtin = struct {
	append  types.Object
	cap     types.Object
	close   types.Object
	complex types.Object
	copy    types.Object
	delete  types.Object
	imag    types.Object
	len     types.Object
	make    types.Object
	new     types.Object
	panic   types.Object
	print   types.Object
	println types.Object
	real    types.Object
	recover types.Object
	true    types.Object
	false   types.Object
	nil     types.Object
}{
	append:  types.Universe.Lookup("append"),
	cap:     types.Universe.Lookup("cap"),
	close:   types.Universe.Lookup("close"),
	complex: types.Universe.Lookup("complex"),
	copy:    types.Universe.Lookup("copy"),
	delete:  types.Universe.Lookup("delete"),
	imag:    types.Universe.Lookup("imag"),
	len:     types.Universe.Lookup("len"),
	make:    types.Universe.Lookup("make"),
	new:     types.Universe.Lookup("new"),
	panic:   types.Universe.Lookup("panic"),
	print:   types.Universe.Lookup("print"),
	println: types.Universe.Lookup("println"),
	real:    types.Universe.Lookup("real"),
	recover: types.Universe.Lookup("recover"),
	true:    types.Universe.Lookup("true"),
	false:   types.Universe.Lookup("false"),
	nil:     types.Universe.Lookup("nil"),
}

func (c *exprCompiler) compileCallExpr(expr *ast.CallExpr) py.Expr {

	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		switch c.ObjectOf(fun) {
		case builtin.make:
			typ := expr.Args[0]
			switch t := typ.(type) {
			case *ast.ArrayType:
				length := expr.Args[1]
				// This is a list comprehension rather than [<nil value>] * length
				// because in the case when T is not a primitive type,
				// every element in the list needs to be a different object.
				return &py.ListComp{
					Elt: c.zeroValue(c.TypeOf(t.Elt)),
					Generators: []py.Comprehension{
						py.Comprehension{
							Target: &py.Name{Id: py.Identifier("_")},
							Iter: &py.Call{
								Func: pyRange,
								Args: []py.Expr{c.compileExpr(length)},
							},
						},
					},
				}
			case *ast.MapType:
				return &py.Dict{}
			default:
				panic(c.err(expr, "bad type in make(): %T", typ))
			}
		case builtin.new:
			typ := expr.Args[0]
			return c.zeroValue(c.TypeOf(typ))
		case builtin.complex:
			return &py.Call{
				Func: pyComplex,
				Args: c.compileExprs(expr.Args),
			}
		case builtin.real:
			return &py.Attribute{Value: c.compileExpr(expr.Args[0]), Attr: py.Identifier("real")}
		case builtin.imag:
			return &py.Attribute{Value: c.compileExpr(expr.Args[0]), Attr: py.Identifier("imag")}
		case builtin.len, builtin.cap:
			t := c.TypeOf(expr.Args[0])
			switch {
			case isString(t):
				return &py.Call{
					Func: pyLen,
					Args: []py.Expr{
						&py.Call{
							Func: &py.Attribute{Value: c.compileExpr(expr.Args[0]), Attr: py.Identifier("encode")},
							Args: []py.Expr{&py.Str{S: `"utf-8"`}},
						},
					},
				}
			default:
				return &py.Call{
					Func: pyLen,
					Args: c.compileExprs(expr.Args),
				}
			}
		}
	case *ast.ArrayType, *ast.ChanType, *ast.FuncType,
		*ast.InterfaceType, *ast.MapType, *ast.StructType:
		// TODO implement type conversions
		return c.compileExpr(expr.Args[0])
	}
	return &py.Call{
		Func: c.compileExpr(expr.Fun),
		Args: c.compileExprs(expr.Args),
	}
}
func (c *exprCompiler) compileSliceExpr(slice *ast.SliceExpr) py.Expr {
	return &py.Subscript{
		Value: c.compileExpr(slice.X),
		Slice: &py.RangeSlice{
			Lower: c.compileExpr(slice.Low),
			Upper: c.compileExpr(slice.High),
		}}
}

func (c *exprCompiler) compileIndexExpr(expr *ast.IndexExpr) py.Expr {
	return &py.Subscript{
		Value: c.compileExpr(expr.X),
		Slice: &py.Index{Value: c.compileExpr(expr.Index)},
	}
}

func (c *exprCompiler) addStmt(stmt py.Stmt) {
	c.stmts = append(c.stmts, stmt)
}

func (c *exprCompiler) compileFuncLit(expr *ast.FuncLit) py.Expr {
	id := c.tempID("func")
	funcDef := c.compileFunc(id, expr.Type, expr.Body, false, nil)
	c.addStmt(funcDef)
	return &py.Name{Id: id}
}

func (c *exprCompiler) compileTypeAssertExpr(expr *ast.TypeAssertExpr) py.Expr {
	// TODO
	return c.compileExpr(expr.X)
}

func (c *exprCompiler) compileStarExpr(expr *ast.StarExpr) py.Expr {
	// TODO
	return c.compileExpr(expr.X)
}

func (c *exprCompiler) compileExpr(expr ast.Expr) py.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		return c.compileUnaryExpr(e)
	case *ast.BinaryExpr:
		return c.compileBinaryExpr(e)
	case *ast.Ident:
		return c.compileIdent(e)
	case *ast.BasicLit:
		return c.compileBasicLit(e)
	case *ast.ParenExpr:
		return c.compileExpr(e.X)
	case *ast.CompositeLit:
		return c.compileCompositeLit(e)
	case *ast.SelectorExpr:
		return c.compileSelectorExpr(e)
	case *ast.CallExpr:
		return c.compileCallExpr(e)
	case *ast.IndexExpr:
		return c.compileIndexExpr(e)
	case *ast.SliceExpr:
		return c.compileSliceExpr(e)
	case *ast.FuncLit:
		return c.compileFuncLit(e)
	case *ast.TypeAssertExpr:
		return c.compileTypeAssertExpr(e)
	case *ast.StarExpr:
		return c.compileStarExpr(e)
	}
	panic(c.err(expr, "unknown Expr: %T", expr))
}

func (c *exprCompiler) compileExprs(exprs []ast.Expr) []py.Expr {
	var pyExprs []py.Expr
	for _, result := range exprs {
		pyExprs = append(pyExprs, c.compileExpr(result))
	}
	return pyExprs
}

func makeTuple(pyExprs []py.Expr) py.Expr {
	switch len(pyExprs) {
	case 0:
		return nil
	case 1:
		return pyExprs[0]
	default:
		return &py.Tuple{Elts: pyExprs}
	}
}

func (c *exprCompiler) compileExprsTuple(exprs []ast.Expr) py.Expr {
	return makeTuple(c.compileExprs(exprs))
}

func (c *exprCompiler) compileCaseClauseTest(caseClause *ast.CaseClause, tag py.Expr) py.Expr {
	var tests []py.Expr
	for _, expr := range caseClause.List {
		var test py.Expr
		if tag != nil {
			test = &py.Compare{
				Left:        tag,
				Ops:         []py.CmpOp{py.Eq},
				Comparators: []py.Expr{c.compileExpr(expr)}}
		} else {
			test = c.compileExpr(expr)
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
