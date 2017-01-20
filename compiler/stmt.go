package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/token"
)

func (c *Compiler) compileStmts(stmts []ast.Stmt) []py.Stmt {
	var pyStmts []py.Stmt
	for _, blockStmt := range stmts {
		pyStmts = append(pyStmts, c.compileStmt(blockStmt)...)
	}
	return pyStmts
}

func (c *Compiler) isBlank(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

func (c *Compiler) compileRangeStmt(stmt *ast.RangeStmt) []py.Stmt {
	e := c.exprCompiler()
	var pyStmt py.Stmt
	if stmt.Key != nil && stmt.Value == nil {
		pyStmt = &py.For{
			Target: e.compileExpr(stmt.Key),
			Iter: &py.Call{
				Func: pyRange,
				Args: []py.Expr{
					&py.Call{
						Func: pyLen,
						Args: []py.Expr{e.compileExpr(stmt.X)},
					},
				}},
			Body: c.compileStmt(stmt.Body),
		}

	} else if stmt.Key != nil && stmt.Value != nil {
		if c.isBlank(stmt.Key) {
			pyStmt = &py.For{
				Target: e.compileExpr(stmt.Value),
				Iter:   e.compileExpr(stmt.X),
				Body:   c.compileStmt(stmt.Body),
			}

		} else {
			pyStmt = &py.For{
				Target: &py.Tuple{Elts: []py.Expr{e.compileExpr(stmt.Key), e.compileExpr(stmt.Value)}},
				Iter: &py.Call{
					Func: pyEnumerate,
					Args: []py.Expr{e.compileExpr(stmt.X)},
				},
				Body: c.compileStmt(stmt.Body),
			}
		}

	} else {
		panic(c.err(stmt, "nil key in range for"))
	}
	return append(e.stmts, pyStmt)
}

func (c *Compiler) compileIncDecStmt(s *ast.IncDecStmt) []py.Stmt {
	e := c.exprCompiler()
	var op py.Operator
	if s.Tok == token.INC {
		op = py.Add
	} else {
		op = py.Sub
	}
	stmt := &py.AugAssign{
		Target: e.compileExpr(s.X),
		Value:  &py.Num{N: "1"},
		Op:     op,
	}
	return append(e.stmts, stmt)
}

func (c *Compiler) compileValueSpec(spec *ast.ValueSpec) []py.Stmt {
	e := c.exprCompiler()
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
		target := c.compileIdent(ident)

		if len(spec.Values) == 0 {
			value := c.zeroValue(c.TypeOf(ident))
			values = append(values, value)
		} else if i < len(spec.Values) {
			value := e.compileExpr(spec.Values[i])
			values = append(values, value)
		}

		targets = append(targets, target)
	}
	stmt := &py.Assign{
		Targets: targets,
		Value:   makeTuple(values),
	}
	return append(e.stmts, stmt)
}

func (c *Compiler) compileDeclStmt(s *ast.DeclStmt) []py.Stmt {
	var stmts []py.Stmt
	genDecl := s.Decl.(*ast.GenDecl)
	for _, spec := range genDecl.Specs {
		var compiled []py.Stmt
		switch spec := spec.(type) {
		case *ast.ValueSpec:
			compiled = c.compileValueSpec(spec)
		case *ast.TypeSpec:
			compiled = []py.Stmt{c.compileTypeSpec(spec)}
		default:
			panic(c.err(s, "unknown Spec: %T", spec))
		}
		stmts = append(stmts, compiled...)
	}
	return stmts
}

func (c *Compiler) augmentedOp(t token.Token) py.Operator {
	switch t {
	case token.ADD_ASSIGN: // +=
		return py.Add
	case token.SUB_ASSIGN: // -=
		return py.Sub
	case token.MUL_ASSIGN: // *=
		return py.Mult
	case token.QUO_ASSIGN: // /=
		return py.FloorDiv
	case token.REM_ASSIGN: // %=
		return py.Mod
	case token.AND_ASSIGN: // &=
		return py.BitAnd
	case token.OR_ASSIGN: // |=
		return py.BitOr
	case token.XOR_ASSIGN: // ^=
		return py.BitXor
	case token.SHL_ASSIGN: // <<=
		return py.LShift
	case token.SHR_ASSIGN: // >>=
		return py.RShift
		// &^= is special cased in compileAssignStmt
	default:
		panic(fmt.Sprintf("augmentedOp bad token %v", t))
	}
}

func (c *Compiler) compileAssignStmt(s *ast.AssignStmt) []py.Stmt {
	e := c.exprCompiler()
	var stmt py.Stmt
	if s.Tok == token.ASSIGN || s.Tok == token.DEFINE {
		stmt = &py.Assign{
			Targets: e.compileExprs(s.Lhs),
			Value:   e.compileExprsTuple(s.Rhs),
		}
	} else if s.Tok == token.AND_NOT_ASSIGN { // x &^= y becomes x &= ~y
		stmt = &py.AugAssign{
			Target: e.compileExpr(s.Lhs[0]),
			Value: &py.UnaryOpExpr{
				Op:      py.Invert,
				Operand: e.compileExpr(s.Rhs[0]),
			},
			Op: py.BitAnd,
		}
	} else {
		stmt = &py.AugAssign{
			Target: e.compileExpr(s.Lhs[0]),
			Value:  e.compileExpr(s.Rhs[0]),
			Op:     c.augmentedOp(s.Tok),
		}
	}
	return append(e.stmts, stmt)
}

func (c *Compiler) compileSwitchStmt(s *ast.SwitchStmt) []py.Stmt {
	e := c.exprCompiler()
	var stmts []py.Stmt
	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	var tag py.Expr
	if s.Tag != nil {
		tag = &py.Name{Id: py.Identifier("tag")}
		assignTag := &py.Assign{Targets: []py.Expr{tag}, Value: e.compileExpr(s.Tag)}
		stmts = append(stmts, assignTag)
	}

	var firstIfStmt *py.If
	var lastIfStmt *py.If
	var defaultBody []py.Stmt
	for _, stmt := range s.Body.List {
		caseClause := stmt.(*ast.CaseClause)
		test := e.compileCaseClauseTest(caseClause, tag)
		if test == nil {
			// no test => default clause
			defaultBody = c.compileStmts(caseClause.Body)
			continue
		}
		ifStmt := &py.If{Test: test, Body: c.compileStmts(caseClause.Body)}
		if firstIfStmt == nil {
			firstIfStmt = ifStmt
			lastIfStmt = ifStmt
		} else {
			lastIfStmt.Orelse = []py.Stmt{ifStmt}
			lastIfStmt = ifStmt
		}
	}
	stmts = append(stmts, e.stmts...)
	if lastIfStmt != nil {
		lastIfStmt.Orelse = defaultBody
		stmts = append(stmts, firstIfStmt)
	} else {
		// no cases apart from default
		stmts = append(stmts, defaultBody...)
	}
	return stmts
}

func (c *Compiler) compileTypeSwitchStmt(s *ast.TypeSwitchStmt) []py.Stmt {
	e := c.exprCompiler()
	var stmts []py.Stmt

	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	var tag py.Expr
	var symbolicVarName string
	var typeAssert ast.Expr

	switch s := s.Assign.(type) {
	case *ast.AssignStmt:
		ident := s.Lhs[0].(*ast.Ident)
		tag = &py.Name{Id: c.tempID(ident.Name)}
		symbolicVarName = ident.Name
		typeAssert = s.Rhs[0]
	case *ast.ExprStmt:
		tag = &py.Name{Id: c.tempID("tag")}
		typeAssert = s.X
	default:
		panic(c.err(s, "Unknown statement type in type switch assign: %T", s))
	}
	expr := typeAssert.(*ast.TypeAssertExpr).X
	tagValue := &py.Call{Func: pyType, Args: []py.Expr{e.compileExpr(expr)}}
	assignTag := &py.Assign{Targets: []py.Expr{tag}, Value: tagValue}
	stmts = append(stmts, assignTag)

	var firstIfStmt *py.If
	var lastIfStmt *py.If
	var defaultBody []py.Stmt
	for _, stmt := range s.Body.List {
		caseClause := stmt.(*ast.CaseClause)
		test := e.compileCaseClauseTest(caseClause, tag)
		var bodyStmts []py.Stmt
		if symbolicVarName != "" {
			typedIdent := c.id(c.Implicits[caseClause])
			assign := &py.Assign{
				Targets: []py.Expr{&py.Name{Id: typedIdent}},
				Value:   tag,
			}
			bodyStmts = append(bodyStmts, assign)
		}
		bodyStmts = append(bodyStmts, c.compileStmts(caseClause.Body)...)
		if test == nil {
			// no test => default clause
			defaultBody = bodyStmts
			continue
		}
		ifStmt := &py.If{Test: test, Body: bodyStmts}
		if firstIfStmt == nil {
			firstIfStmt = ifStmt
			lastIfStmt = ifStmt
		} else {
			lastIfStmt.Orelse = []py.Stmt{ifStmt}
			lastIfStmt = ifStmt
		}
	}
	stmts = append(stmts, e.stmts...)
	if lastIfStmt != nil {
		lastIfStmt.Orelse = defaultBody
		stmts = append(stmts, firstIfStmt)
	} else {
		// no cases apart from default
		stmts = append(stmts, defaultBody...)
	}
	return stmts
}

func (c *Compiler) compileIfStmt(s *ast.IfStmt) []py.Stmt {
	e := c.exprCompiler()
	var stmts []py.Stmt
	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	ifStmt := &py.If{Test: e.compileExpr(s.Cond), Body: c.compileStmt(s.Body)}
	if s.Else != nil {
		ifStmt.Orelse = c.compileStmt(s.Else)
	}
	stmts = append(stmts, e.stmts...)
	stmts = append(stmts, ifStmt)
	return stmts
}

func (c *Compiler) compileBranchStmt(s *ast.BranchStmt) []py.Stmt {
	switch s.Tok {
	case token.BREAK:
		return []py.Stmt{&py.Break{}}
	case token.CONTINUE:
		return []py.Stmt{&py.Continue{}}
	case token.FALLTHROUGH:
		return []py.Stmt{&py.ExprStmt{Value: &py.Call{Func: &py.Name{Id: py.Identifier("_TODO_fallthrough")}}}}
	default:
		panic(c.err(s, "unknown BranchStmt %v", s.Tok))
	}
}

func (c *Compiler) compileForStmt(s *ast.ForStmt) []py.Stmt {
	e := c.exprCompiler()
	var stmts []py.Stmt
	body := c.compileStmt(s.Body)
	if s.Post != nil {
		body = append(c.compileStmt(s.Body), c.compileStmt(s.Post)...)
	}
	if s.Init != nil {
		stmts = c.compileStmt(s.Init)
	}
	var test py.Expr = pyTrue
	if s.Cond != nil {
		test = e.compileExpr(s.Cond)
	}
	stmts = append(stmts, e.stmts...)
	stmts = append(stmts, &py.While{Test: test, Body: body})
	return stmts
}

func (c *Compiler) compileExprToStmt(e ast.Expr) []py.Stmt {
	ec := c.exprCompiler()
	var stmt py.Stmt
	switch e := e.(type) {
	case *ast.CallExpr:
		switch fun := e.Fun.(type) {
		case *ast.Ident:
			switch fun.Name {
			case "delete":
				stmt = &py.Try{
					Body: []py.Stmt{
						&py.Delete{
							Targets: []py.Expr{
								&py.Subscript{
									Value: ec.compileExpr(e.Args[0]),
									Slice: &py.Index{ec.compileExpr(e.Args[1])},
								},
							},
						},
					},
					Handlers: []py.ExceptHandler{
						py.ExceptHandler{
							Typ:  pyKeyError,
							Body: []py.Stmt{&py.Pass{}},
						},
					},
				}
			}
		}
	}
	if stmt == nil {
		return nil
	}
	return append(ec.stmts, stmt)
}

func (c *Compiler) compileReturnStmt(s *ast.ReturnStmt) []py.Stmt {
	e := c.exprCompiler()
	stmt := &py.Return{Value: e.compileExprsTuple(s.Results)}
	return append(e.stmts, stmt)
}

func (c *Compiler) compileExprStmt(s *ast.ExprStmt) []py.Stmt {
	if compiled := c.compileExprToStmt(s.X); compiled != nil {
		return compiled
	}
	e := c.exprCompiler()
	stmt := &py.ExprStmt{Value: e.compileExpr(s.X)}
	return append(e.stmts, stmt)
}

func (c *Compiler) compileStmt(stmt ast.Stmt) []py.Stmt {

	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return c.compileReturnStmt(s)
	case *ast.ForStmt:
		return c.compileForStmt(s)
	case *ast.BlockStmt:
		return c.compileStmts(s.List)
	case *ast.AssignStmt:
		return c.compileAssignStmt(s)
	case *ast.ExprStmt:
		return c.compileExprStmt(s)
	case *ast.RangeStmt:
		return c.compileRangeStmt(s)
	case *ast.IfStmt:
		return c.compileIfStmt(s)
	case *ast.IncDecStmt:
		return c.compileIncDecStmt(s)
	case *ast.DeclStmt:
		return c.compileDeclStmt(s)
	case *ast.SwitchStmt:
		return c.compileSwitchStmt(s)
	case *ast.TypeSwitchStmt:
		return c.compileTypeSwitchStmt(s)
	case *ast.BranchStmt:
		return c.compileBranchStmt(s)
	case *ast.EmptyStmt:
		return []py.Stmt{}
	case *ast.DeferStmt:
		// TODO
		return []py.Stmt{}
	}
	panic(c.err(stmt, "unknown Stmt: %T", stmt))
}
