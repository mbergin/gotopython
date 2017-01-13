package pythonast

import (
	"fmt"
	"io"
)

type Writer struct {
	out         io.Writer
	indentLevel int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{out: w}
}

func (w *Writer) WriteModule(m *Module) {
	for _, bodyStmt := range m.Body {
		w.writeStmt(bodyStmt)
		w.newline()
	}
}

func (w *Writer) writeStmts(stmts []Stmt) {
	for i, stmt := range stmts {
		if i > 0 {
			w.newline()
		}
		w.writeStmt(stmt)
	}
}

func (w *Writer) writeStmt(stmt Stmt) {
	switch s := stmt.(type) {
	case *FunctionDef:
		w.functionDef(s)
	case *ClassDef:
		w.classDef(s)
	case *While:
		w.write("while ")
		w.writeExpr(s.Test)
		w.write(":")
		w.indent()
		w.writeStmts(s.Body)
		w.dedent()
	case *Assign:
		for i, target := range s.Targets {
			if i > 0 {
				w.comma()
			}
			w.writeExpr(target)
		}
		w.write(" = ")
		w.writeExpr(s.Value)
	case *Return:
		if s.Value != nil {
			w.write("return ")
			w.writeExpr(s.Value)
		} else {
			w.write("return")
		}
	case *Pass:
		w.write("pass")
	case *ExprStmt:
		w.writeExpr(s.Value)
	case *If:
		w.write("if ")
		w.writeExpr(s.Test)
		w.write(":")
		w.indent()
		w.writeStmts(s.Body)
		w.dedent()
		if s.Orelse != nil {
			if elif, ok := s.Orelse[0].(*If); ok {
				w.write("el")
				w.writeStmt(elif)
			} else {
				w.write("else:")
				w.indent()
				w.writeStmts(s.Orelse)
				w.dedent()
			}
		}
	case *AugAssign:
		w.augAssign(s)
	case *For:
		w.write("for ")
		w.writeExpr(s.Target)
		w.write(" in ")
		w.writeExpr(s.Iter)
		w.write(":")
		w.indent()
		for i, bodyStmt := range s.Body {
			if i > 0 {
				w.newline()
			}
			w.writeStmt(bodyStmt)
		}
		w.dedent()
	case *Break:
		w.write("break")
	case *Continue:
		w.write("continue")
	case *Delete:
		w.write("del ")
		for i, target := range s.Targets {
			if i > 0 {
				w.comma()
			}
			w.writeExpr(target)
		}
	case *Try:
		w.try(s)
	default:
		panic(fmt.Sprintf("unknown Stmt: %T", stmt))
	}
}

func (w *Writer) try(s *Try) {
	w.write("try:")
	w.indent()
	w.writeStmts(s.Body)
	w.dedent()
	for _, handler := range s.Handlers {
		w.write("except")
		if handler.Typ != nil {
			w.write(" ")
			w.writeExpr(handler.Typ)
			if handler.Name != Identifier("") {
				w.write(" as ")
				w.identifier(handler.Name)
			}
		}
		w.write(":")
		w.indent()
		w.writeStmts(handler.Body)
		w.dedent()
	}
	if len(s.Orelse) > 0 {
		w.write("else:")
		w.indent()
		w.writeStmts(s.Orelse)
		w.dedent()
	}
}

func (w *Writer) augAssign(s *AugAssign) {
	w.writeExpr(s.Target)
	w.write(" ")
	w.writeOp(s.Op)
	w.write("=")
	w.write(" ")
	w.writeExpr(s.Value)
}

func (w *Writer) writeExprPrec(expr Expr, parentPrec int) {
	if expr == nil {
		panic("nil expr")
	}
	prec := expr.Precedence()
	paren := prec < parentPrec
	if paren {
		w.beginParen()
	}
	switch e := expr.(type) {
	case *BinOp:
		w.writeExprPrec(e.Left, prec)
		w.writeOp(e.Op)
		w.writeExprPrec(e.Right, prec)
	case *Name:
		w.identifier(e.Id)
	case *Num:
		w.write(e.N)
	case *Str:
		w.write(e.S)
	case *Compare:
		w.writeExprPrec(e.Left, prec)
		for i := range e.Ops {
			w.writeCmpOp(e.Ops[i])
			w.writeExprPrec(e.Comparators[i], prec)
		}
	case *Tuple:
		for i, elt := range e.Elts {
			if i > 0 {
				w.comma()
			}
			w.writeExprPrec(elt, prec)
		}
	case *Call:
		w.writeExprPrec(e.Func, prec)
		w.beginParen()
		i := 0
		for _, arg := range e.Args {
			if i != 0 {
				w.comma()
			}
			w.writeExprPrec(arg, prec)
			i++
		}
		for _, kw := range e.Keywords {
			if i != 0 {
				w.comma()
			}
			w.identifier(*kw.Arg)
			w.write("=")
			w.writeExprPrec(kw.Value, prec)
			i++
		}
		w.endParen()
	case *Attribute:
		w.writeExprPrec(e.Value, prec)
		w.write(".")
		w.identifier(e.Attr)
	case *NameConstant:
		w.nameConstant(e)
	case *List:
		w.list(e)
	case *Subscript:
		w.writeExprPrec(e.Value, prec)
		w.write("[")
		w.slice(e.Slice)
		w.write("]")
	case *BoolOpExpr:
		w.boolOpExpr(e)
	case *UnaryOpExpr:
		w.unaryOpExpr(e)
	case *ListComp:
		w.listComp(e)
	default:
		panic(fmt.Sprintf("unknown Expr: %T", expr))
	}
	if paren {
		w.endParen()
	}
}

func (w *Writer) listComp(e *ListComp) {
	w.write("[")
	w.writeExpr(e.Elt)
	for _, g := range e.Generators {
		w.write(" for ")
		w.writeExpr(g.Target)
		w.write(" in ")
		w.writeExpr(g.Iter)
		for _, ifExpr := range g.Ifs {
			w.write(" if ")
			w.writeExpr(ifExpr)
		}
	}
	w.write("]")
}

func (w *Writer) boolOpExpr(e *BoolOpExpr) {
	w.writeExprPrec(e.Values[0], e.Precedence())
	switch e.Op {
	case Or:
		w.write(" or ")
	case And:
		w.write(" and ")
	}
	w.writeExprPrec(e.Values[1], e.Precedence())
}

func (w *Writer) unaryOpExpr(e *UnaryOpExpr) {
	switch e.Op {
	case Invert:
		w.write("~")
	case Not:
		w.write("not ")
	case UAdd:
		w.write("+")
	case USub:
		w.write("-")
	}
	w.writeExprPrec(e.Operand, e.Precedence())
}

func (w *Writer) slice(s Slice) {
	switch s := s.(type) {
	case *Index:
		w.writeExpr(s.Value)
	case *RangeSlice:
		if s.Lower != nil {
			w.writeExpr(s.Lower)
		}
		w.write(":")
		if s.Upper != nil {
			w.writeExpr(s.Upper)
		}
	default:
		panic(fmt.Sprintf("unknown Slice: %T", s))
	}
}

func (w *Writer) list(l *List) {
	w.write("[")
	for i, elt := range l.Elts {
		if i > 0 {
			w.comma()
		}
		w.writeExprPrec(elt, l.Precedence())
	}
	w.write("]")
}

func (w *Writer) nameConstant(nc *NameConstant) {
	switch nc.Value {
	case None:
		w.write("None")
	case True:
		w.write("True")
	case False:
		w.write("False")
	default:
		panic(fmt.Sprintf("unknown NameConstant %v", nc.Value))
	}
}

func (w *Writer) writeExpr(expr Expr) {
	w.writeExprPrec(expr, 0)
}

func (w *Writer) writeOp(op Operator) {
	switch op {
	case Add:
		w.write("+")
	case Sub:
		w.write("-")
	case Mult:
		w.write("*")
	case MatMult:
		w.write("@")
	case Div:
		w.write("/")
	case Mod:
		w.write("%")
	case Pow:
		w.write("**")
	case LShift:
		w.write("<<")
	case RShift:
		w.write(">>")
	case BitOr:
		w.write("|")
	case BitXor:
		w.write("^")
	case BitAnd:
		w.write("&")
	case FloorDiv:
		w.write("//")
	}
}

func (w *Writer) writeCmpOp(op CmpOp) {
	switch op {
	case Eq:
		w.write("==")
	case NotEq:
		w.write("!=")
	case Lt:
		w.write("<")
	case LtE:
		w.write("<=")
	case Gt:
		w.write(">")
	case GtE:
		w.write(">=")
	case Is:
		w.write(" is ")
	case IsNot:
		w.write(" is not ")
	case In:
		w.write(" in ")
	case NotIn:
		w.write(" not in ")
	}
}

func (w *Writer) functionDef(s *FunctionDef) {
	w.write("def ")
	w.identifier(s.Name)
	w.beginParen()
	defaultOffset := len(s.Args.Args) - len(s.Args.Defaults)
	for i, arg := range s.Args.Args {
		if i > 0 {
			w.comma()
		}
		w.identifier(arg.Arg)
		if i >= defaultOffset {
			w.write("=")
			w.writeExpr(s.Args.Defaults[i-defaultOffset])
		}
	}
	w.endParen()
	w.write(":")
	w.indent()
	for i, bodyStmt := range s.Body {
		if i > 0 {
			w.newline()
		}
		w.writeStmt(bodyStmt)
	}
	w.dedent()
}

func (w *Writer) classDef(s *ClassDef) {
	w.write("class ")
	w.identifier(s.Name)
	if len(s.Bases) > 0 {
		w.beginParen()
		for i, base := range s.Bases {
			if i > 0 {
				w.comma()
			}
			w.writeExpr(base)
		}
		w.endParen()
	}
	w.write(":")
	w.indent()
	for i, bodyStmt := range s.Body {
		if i > 0 {
			w.newline()
		}
		w.writeStmt(bodyStmt)
	}
	w.dedent()
}

func (w *Writer) identifier(i Identifier) {
	w.write(string(i))
}

func (w *Writer) comma() {
	w.write(", ")
}

func (w *Writer) beginParen() {
	w.write("(")
}

func (w *Writer) endParen() {
	w.write(")")
}

func (w *Writer) indent() {
	w.indentLevel++
	w.newline()
}

func (w *Writer) newline() {
	w.write("\n")
	for i := 0; i < w.indentLevel; i++ {
		w.write("    ")
	}
}

func (w *Writer) dedent() {
	w.indentLevel--
	w.newline()
}

func (w *Writer) write(s string) {
	w.out.Write([]byte(s))
}
