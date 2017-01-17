package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func (c *Compiler) compileIdent(ident *ast.Ident) py.Expr {
	switch ident.Name {
	case "true":
		return pyTrue
	case "false":
		return pyFalse
	case "nil":
		return pyNone
	default:
		return &py.Name{Id: py.Identifier(ident.Name)}
	}
}

func (c *Compiler) comparator(t token.Token) (py.CmpOp, bool) {
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

func (c *Compiler) binOp(t token.Token) (py.Operator, bool) {
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

func (c *Compiler) boolOp(t token.Token) (py.BoolOp, bool) {
	switch t {
	case token.LAND:
		return py.And, true
	case token.LOR:
		return py.Or, true
	}
	return py.BoolOp(0), false
}

func (c *Compiler) compileBinaryExpr(expr *ast.BinaryExpr) py.Expr {
	if pyCmp, ok := c.comparator(expr.Op); ok {
		return &py.Compare{
			Left:        c.compileExpr(expr.X),
			Ops:         []py.CmpOp{pyCmp},
			Comparators: []py.Expr{c.compileExpr(expr.Y)}}
	}
	if pyOp, ok := c.binOp(expr.Op); ok {
		return &py.BinOp{Left: c.compileExpr(expr.X),
			Right: c.compileExpr(expr.Y),
			Op:    pyOp}
	}
	if pyBoolOp, ok := c.boolOp(expr.Op); ok {
		return &py.BoolOpExpr{
			Values: []py.Expr{c.compileExpr(expr.X), c.compileExpr(expr.Y)},
			Op:     pyBoolOp}
	}
	if expr.Op == token.AND_NOT {
		return &py.BinOp{Left: c.compileExpr(expr.X),
			Right: &py.UnaryOpExpr{Op: py.Invert, Operand: c.compileExpr(expr.Y)},
			Op:    py.BitAnd}
	}
	panic(fmt.Sprintf("unknown BinaryExpr Op: %v", expr.Op))
}

func (c *Compiler) compileBasicLit(expr *ast.BasicLit) py.Expr {
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
	panic(fmt.Sprintf("unknown BasicLit kind: %v", expr.Kind))
}

func (c *Compiler) compileUnaryExpr(expr *ast.UnaryExpr) py.Expr {
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
	panic(fmt.Sprintf("unknown UnaryExpr: %v", expr.Op))
}

func (c *Compiler) compileCompositeLit(expr *ast.CompositeLit, parentElementType ast.Expr) py.Expr {
	var clType ast.Expr
	// Allowed to omit the type if this is an element of another composite literal
	if expr.Type == nil {
		clType = parentElementType
	} else {
		clType = expr.Type
	}
	switch typ := clType.(type) {
	case *ast.Ident:
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
			Func:     c.compileIdent(typ),
			Args:     args,
			Keywords: keywords,
		}
	case *ast.ArrayType:
		elts := make([]py.Expr, len(expr.Elts))
		for i, elt := range expr.Elts {
			if cl, ok := elt.(*ast.CompositeLit); ok {
				elts[i] = c.compileCompositeLit(cl, typ.Elt)
			} else {
				elts[i] = c.compileExpr(elt)
			}
		}
		return &py.List{Elts: elts}
	case *ast.MapType:
		keys := make([]py.Expr, len(expr.Elts))
		values := make([]py.Expr, len(expr.Elts))
		for i, elt := range expr.Elts {
			kv := elt.(*ast.KeyValueExpr)
			if clKey, ok := kv.Key.(*ast.CompositeLit); ok {
				keys[i] = c.compileCompositeLit(clKey, typ.Key)
			} else {
				keys[i] = c.compileExpr(kv.Key)
			}
			if clValue, ok := kv.Value.(*ast.CompositeLit); ok {
				values[i] = c.compileCompositeLit(clValue, typ.Value)
			} else {
				values[i] = c.compileExpr(kv.Value)
			}
		}
		return &py.Dict{Keys: keys, Values: values}
	default:
		panic(fmt.Sprintf("Unknown composite literal type: %T", expr.Type))
	}
}

func (c *Compiler) compileSelectorExpr(expr *ast.SelectorExpr) py.Expr {
	return &py.Attribute{
		Value: c.compileExpr(expr.X),
		Attr:  c.identifier(expr.Sel),
	}
}

func isString(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsString != 0
}

func (c *Compiler) compileCallExpr(expr *ast.CallExpr) py.Expr {
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		// TODO need to use proper name resolution to make sure these
		// are really calls to builtin functions and not user-defined
		// functions that hide them.
		case "make":
			typ := expr.Args[0]
			switch t := typ.(type) {
			case *ast.ArrayType:
				length := expr.Args[1]
				// This is a list comprehension rather than [<nil value>] * length
				// because in the case when T is not a primitive type,
				// every element in the list needs to be a different object.
				return &py.ListComp{
					Elt: c.nilValue(t.Elt),
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
				panic("bad type in make()")
			}
		case "new":
			typ := expr.Args[0]
			return c.nilValue(typ)
		case "complex":
			return &py.Call{
				Func: pyComplex,
				Args: c.compileExprs(expr.Args),
			}
		case "real":
			return &py.Attribute{Value: c.compileExpr(expr.Args[0]), Attr: py.Identifier("real")}
		case "imag":
			return &py.Attribute{Value: c.compileExpr(expr.Args[0]), Attr: py.Identifier("imag")}
		case "len", "cap":
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
func (c *Compiler) compileSliceExpr(slice *ast.SliceExpr) py.Expr {
	return &py.Subscript{
		Value: c.compileExpr(slice.X),
		Slice: &py.RangeSlice{
			Lower: c.compileExpr(slice.Low),
			Upper: c.compileExpr(slice.High),
		}}
}

func (c *Compiler) compileIndexExpr(expr *ast.IndexExpr) py.Expr {
	return &py.Subscript{
		Value: c.compileExpr(expr.X),
		Slice: &py.Index{Value: c.compileExpr(expr.Index)},
	}
}

func (c *Compiler) compileExpr(expr ast.Expr) py.Expr {
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
		return c.compileCompositeLit(e, nil)
	case *ast.SelectorExpr:
		return c.compileSelectorExpr(e)
	case *ast.CallExpr:
		return c.compileCallExpr(e)
	case *ast.IndexExpr:
		return c.compileIndexExpr(e)
	case *ast.SliceExpr:
		return c.compileSliceExpr(e)
	}
	panic(fmt.Sprintf("unknown Expr: %T", expr))
}

func (c *Compiler) compileExprs(exprs []ast.Expr) []py.Expr {
	var pyExprs []py.Expr
	for _, result := range exprs {
		pyExprs = append(pyExprs, c.compileExpr(result))
	}
	return pyExprs
}

func (c *Compiler) makeTuple(pyExprs []py.Expr) py.Expr {
	switch len(pyExprs) {
	case 0:
		return nil
	case 1:
		return pyExprs[0]
	default:
		return &py.Tuple{Elts: pyExprs}
	}
}

func (c *Compiler) compileExprsTuple(exprs []ast.Expr) py.Expr {
	return c.makeTuple(c.compileExprs(exprs))
}
