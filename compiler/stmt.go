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

var (
	pyRange     = &py.Name{Id: py.Identifier("range")}
	pyLen       = &py.Name{Id: py.Identifier("len")}
	pyEnumerate = &py.Name{Id: py.Identifier("enumerate")}
	pyType      = &py.Name{Id: py.Identifier("type")}
	pyKeyError  = &py.Name{Id: py.Identifier("KeyError")}
)

func (c *Compiler) isBlank(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

func (c *Compiler) compileRangeStmt(stmt *ast.RangeStmt) py.Stmt {
	if stmt.Key != nil && stmt.Value == nil {
		return &py.For{
			Target: c.compileExpr(stmt.Key),
			Iter: &py.Call{
				Func: pyRange,
				Args: []py.Expr{
					&py.Call{
						Func: pyLen,
						Args: []py.Expr{c.compileExpr(stmt.X)},
					},
				}},
			Body: c.compileStmt(stmt.Body),
		}
	}
	if stmt.Key != nil && stmt.Value != nil {
		if c.isBlank(stmt.Key) {
			return &py.For{
				Target: c.compileExpr(stmt.Value),
				Iter:   c.compileExpr(stmt.X),
				Body:   c.compileStmt(stmt.Body),
			}
		}
		return &py.For{
			Target: &py.Tuple{Elts: []py.Expr{c.compileExpr(stmt.Key), c.compileExpr(stmt.Value)}},
			Iter: &py.Call{
				Func: pyEnumerate,
				Args: []py.Expr{c.compileExpr(stmt.X)},
			},
			Body: c.compileStmt(stmt.Body),
		}
	}
	panic("nil key in range for")
}

func (c *Compiler) compileIncDecStmt(s *ast.IncDecStmt) py.Stmt {
	var op py.Operator
	if s.Tok == token.INC {
		op = py.Add
	} else {
		op = py.Sub
	}
	return &py.AugAssign{
		Target: c.compileExpr(s.X),
		Value:  &py.Num{N: "1"},
		Op:     op,
	}
}

func (c *Compiler) compileValueSpec(spec *ast.ValueSpec) []py.Stmt {
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
			value := c.nilValue(spec.Type)
			values = append(values, value)
		} else if i < len(spec.Values) {
			value := c.compileExpr(spec.Values[i])
			values = append(values, value)
		}

		targets = append(targets, target)
	}
	return []py.Stmt{
		&py.Assign{
			Targets: targets,
			Value:   c.makeTuple(values),
		},
	}
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
			panic(fmt.Sprintf("unknown Spec: %T", spec))
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

func (c *Compiler) compileAssignStmt(s *ast.AssignStmt) py.Stmt {
	if s.Tok == token.ASSIGN || s.Tok == token.DEFINE {
		return &py.Assign{
			Targets: c.compileExprs(s.Lhs),
			Value:   c.compileExprsTuple(s.Rhs),
		}
	}
	// x &^= y becomes x &= ~y
	if s.Tok == token.AND_NOT_ASSIGN {
		return &py.AugAssign{
			Target: c.compileExpr(s.Lhs[0]),
			Value: &py.UnaryOpExpr{
				Op:      py.Invert,
				Operand: c.compileExpr(s.Rhs[0]),
			},
			Op: py.BitAnd,
		}
	}
	return &py.AugAssign{
		Target: c.compileExpr(s.Lhs[0]),
		Value:  c.compileExpr(s.Rhs[0]),
		Op:     c.augmentedOp(s.Tok),
	}
}

func (c *Compiler) compileCaseClauseTest(caseClause *ast.CaseClause, tag py.Expr) py.Expr {
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

func (c *Compiler) compileSwitchStmt(s *ast.SwitchStmt) []py.Stmt {
	var stmts []py.Stmt
	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	var tag py.Expr
	if s.Tag != nil {
		tag = &py.Name{Id: py.Identifier("tag")}
		assignTag := &py.Assign{Targets: []py.Expr{tag}, Value: c.compileExpr(s.Tag)}
		stmts = append(stmts, assignTag)
	}

	var firstIfStmt *py.If
	var lastIfStmt *py.If
	var defaultBody []py.Stmt
	for _, stmt := range s.Body.List {
		caseClause := stmt.(*ast.CaseClause)
		test := c.compileCaseClauseTest(caseClause, tag)
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
	var stmts []py.Stmt

	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	var tag py.Expr
	var typeAssert ast.Expr
	switch s := s.Assign.(type) {
	case *ast.AssignStmt:
		tag = c.compileExpr(s.Lhs[0])
		typeAssert = s.Rhs[0]
	case *ast.ExprStmt:
		tag = &py.Name{Id: py.Identifier("tag")}
		typeAssert = s.X
	default:
		panic(fmt.Sprintf("Unknown statement type in type switch assign: %T", s))
	}
	expr := typeAssert.(*ast.TypeAssertExpr).X
	tagValue := &py.Call{Func: pyType, Args: []py.Expr{c.compileExpr(expr)}}
	assignTag := &py.Assign{Targets: []py.Expr{tag}, Value: tagValue}
	stmts = append(stmts, assignTag)

	var firstIfStmt *py.If
	var lastIfStmt *py.If
	var defaultBody []py.Stmt
	for _, stmt := range s.Body.List {
		caseClause := stmt.(*ast.CaseClause)
		test := c.compileCaseClauseTest(caseClause, tag)
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
	var stmts []py.Stmt
	if s.Init != nil {
		stmts = append(stmts, c.compileStmt(s.Init)...)
	}
	ifStmt := &py.If{Test: c.compileExpr(s.Cond), Body: c.compileStmt(s.Body)}
	if s.Else != nil {
		ifStmt.Orelse = c.compileStmt(s.Else)
	}
	stmts = append(stmts, ifStmt)
	return stmts
}

func (c *Compiler) compileBranchStmt(s *ast.BranchStmt) py.Stmt {
	switch s.Tok {
	case token.BREAK:
		return &py.Break{}
	case token.CONTINUE:
		return &py.Continue{}
	default:
		panic(fmt.Sprintf("unknown BranchStmt %v", s.Tok))
	}
}

func (c *Compiler) compileForStmt(s *ast.ForStmt) []py.Stmt {
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
		test = c.compileExpr(s.Cond)
	}
	stmts = append(stmts, &py.While{Test: test, Body: body})
	return stmts
}

func (c *Compiler) compileExprToStmt(e ast.Expr) []py.Stmt {
	switch e := e.(type) {
	case *ast.CallExpr:
		switch fun := e.Fun.(type) {
		case *ast.Ident:
			switch fun.Name {
			case "delete":
				return []py.Stmt{
					&py.Try{
						Body: []py.Stmt{
							&py.Delete{
								Targets: []py.Expr{
									&py.Subscript{
										Value: c.compileExpr(e.Args[0]),
										Slice: &py.Index{c.compileExpr(e.Args[1])},
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
					},
				}
			}
		}
	}
	return nil
}

func (c *Compiler) compileStmt(stmt ast.Stmt) []py.Stmt {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return []py.Stmt{&py.Return{Value: c.compileExprsTuple(s.Results)}}
	case *ast.ForStmt:
		return c.compileForStmt(s)
	case *ast.BlockStmt:
		return c.compileStmts(s.List)
	case *ast.AssignStmt:
		return []py.Stmt{c.compileAssignStmt(s)}
	case *ast.ExprStmt:
		if compiled := c.compileExprToStmt(s.X); compiled != nil {
			return compiled
		}
		return []py.Stmt{&py.ExprStmt{Value: c.compileExpr(s.X)}}
	case *ast.RangeStmt:
		return []py.Stmt{c.compileRangeStmt(s)}
	case *ast.IfStmt:
		return c.compileIfStmt(s)
	case *ast.IncDecStmt:
		return []py.Stmt{c.compileIncDecStmt(s)}
	case *ast.DeclStmt:
		return c.compileDeclStmt(s)
	case *ast.SwitchStmt:
		return c.compileSwitchStmt(s)
	case *ast.TypeSwitchStmt:
		return c.compileTypeSwitchStmt(s)
	case *ast.BranchStmt:
		return []py.Stmt{c.compileBranchStmt(s)}
	case *ast.EmptyStmt:
		return []py.Stmt{}
	}
	panic(fmt.Sprintf("unknown Stmt: %T", stmt))
}
