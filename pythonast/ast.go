package pythonast

// See https://hg.python.org/cpython/file/tip/Parser/Python.asdl

type Module struct {
	Body []Stmt
}

type Stmt interface {
	stmt()
}

func (FunctionDef) stmt()      {}
func (AsyncFunctionDef) stmt() {}
func (ClassDef) stmt()         {}
func (Return) stmt()           {}
func (Delete) stmt()           {}
func (Assign) stmt()           {}
func (AugAssign) stmt()        {}
func (AnnAssign) stmt()        {}
func (For) stmt()              {}
func (AsyncFor) stmt()         {}
func (While) stmt()            {}
func (If) stmt()               {}
func (With) stmt()             {}
func (AsyncWith) stmt()        {}
func (Raise) stmt()            {}
func (Try) stmt()              {}
func (Assert) stmt()           {}
func (Import) stmt()           {}
func (ImportFrom) stmt()       {}
func (Global) stmt()           {}
func (Nonlocal) stmt()         {}
func (ExprStmt) stmt()         {}
func (Pass) stmt()             {}
func (Break) stmt()            {}
func (Continue) stmt()         {}

type Expr interface {
	Precedence() int
}

type FunctionDef struct {
	Name          Identifier
	Args          Arguments
	Body          []Stmt
	DecoratorList []Expr
	Returns       Expr
}

type AsyncFunctionDef struct {
	Name          Identifier
	Args          Arguments
	Body          []Stmt
	DecoratorList []Expr
	Returns       Expr
}

type ClassDef struct {
	Name          Identifier
	Bases         []Expr
	Keywords      []Keyword
	Body          []Stmt
	DecoratorList []Expr
}

type Return struct{ Value Expr }

type Delete struct{ Targets []Expr }
type Assign struct {
	Targets []Expr
	Value   Expr
}
type AugAssign struct {
	Target Expr
	Op     Operator
	Value  Expr
}

// 'simple' indicates that we annotate simple name without parens
type AnnAssign struct {
	Target     Expr
	Annotation Expr
	Value      Expr
	Simple     bool
}

// use 'orelse' because else is a keyword in target languages
type For struct {
	Target Expr
	Iter   Expr
	Body   []Stmt
	Orelse []Stmt
}
type AsyncFor struct {
	Target Expr
	Iter   Expr
	Body   []Stmt
	Orelse []Stmt
}
type While struct {
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}
type If struct {
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}
type With struct {
	Items []WithItem
	Body  []Stmt
}
type AsyncWith struct {
	Items []WithItem
	Body  []Stmt
}

type Raise struct {
	Exc   Expr
	Cause Expr
}
type Try struct {
	Body      []Stmt
	Handlers  []ExceptHandler
	Orelse    []Stmt
	Finalbody []Stmt
}
type Assert struct {
	Test Expr
	Msg  Expr
}

type Import struct{ Names []Alias }
type ImportFrom struct {
	Module *Identifier
	Names  []Alias
	Level  *int
}

type Global struct{ Names []Identifier }
type Nonlocal struct{ Names []Identifier }
type ExprStmt struct{ Value Expr }
type Pass struct{}
type Break struct{}
type Continue struct{}

func (Lambda) Precedence() int       { return 1 }
func (IfExp) Precedence() int        { return 2 }
func (e BoolOpExpr) Precedence() int { return e.Op.Precedence() }
func (o BoolOp) Precedence() int {
	switch o {
	case Or:
		return 3
	case And:
		return 4
	}
	panic("unknown BoolOp")
}

func (Compare) Precedence() int { return 6 }

func (e BinOp) Precedence() int { return e.Op.Precedence() }
func (o Operator) Precedence() int {
	switch o {
	case BitOr:
		return 7
	case BitXor:
		return 8
	case BitAnd:
		return 9
	case LShift, RShift:
		return 10
	case Add, Sub:
		return 11
	case Mult, MatMult, FloorDiv, Div, Mod:
		return 12
	case Pow:
		return 14
	}
	panic("unknown Operator")
}

func (e UnaryOpExpr) Precedence() int { return e.Op.Precedence() }
func (o UnaryOp) Precedence() int {
	switch o {
	case Not:
		return 5
	case Invert, UAdd, USub:
		return 13
	}
	panic("unknown UnaryOp")
}

func (Await) Precedence() int { return 15 }

func (Subscript) Precedence() int { return 16 }
func (Call) Precedence() int      { return 16 }
func (Attribute) Precedence() int { return 16 }

func (Dict) Precedence() int  { return 17 }
func (Set) Precedence() int   { return 17 }
func (List) Precedence() int  { return 17 }
func (Tuple) Precedence() int { return 17 }

func (ListComp) Precedence() int     { return 17 }
func (SetComp) Precedence() int      { return 17 }
func (DictComp) Precedence() int     { return 17 }
func (GeneratorExp) Precedence() int { return 17 }

func (Yield) Precedence() int     { return 0 } // dubious
func (YieldFrom) Precedence() int { return 0 }

func (Num) Precedence() int            { return 100 }
func (Str) Precedence() int            { return 100 }
func (FormattedValue) Precedence() int { return 0 }
func (JoinedStr) Precedence() int      { return 100 }
func (Bytes) Precedence() int          { return 100 }
func (NameConstant) Precedence() int   { return 100 }
func (Ellipsis) Precedence() int       { return 0 }
func (ConstantExpr) Precedence() int   { return 100 }

func (Starred) Precedence() int { return 0 }
func (Name) Precedence() int    { return 100 }

type BoolOpExpr struct {
	Op     BoolOp
	Values []Expr
}
type BinOp struct {
	Left  Expr
	Op    Operator
	Right Expr
}
type UnaryOpExpr struct {
	Op      UnaryOp
	Operand Expr
}
type Lambda struct {
	Args Arguments
	Body Expr
}
type IfExp struct {
	Test   Expr
	Body   Expr
	Orelse Expr
}
type Dict struct {
	Keys   []Expr
	Values []Expr
}
type Set struct{ Elts []Expr }
type ListComp struct {
	Elt        Expr
	Generators []Comprehension
}
type SetComp struct {
	Elt        Expr
	Generators []Comprehension
}
type DictComp struct {
	Key        Expr
	Value      Expr
	Generators []Comprehension
}
type GeneratorExp struct {
	Elt        Expr
	Generators []Comprehension
}

// the grammar constrains where yield expressions can occur
type Await struct{ Value Expr }
type Yield struct{ Value Expr }
type YieldFrom struct{ Value Expr }

// need sequences for compare to distinguish between
// x < 4 < 3 and  struct {< x 4} < 3
type Compare struct {
	Left        Expr
	Ops         []CmpOp
	Comparators []Expr
}
type Call struct {
	Func     Expr
	Args     []Expr
	Keywords []Keyword
}
type Num struct{ N string } // a number as a PyObject.
type Str struct{ S string } // need to specify raw; unicode; *etc
type FormattedValue struct {
	Value      Expr
	Conversion *int
	FormatSpec Expr
}
type JoinedStr struct{ Values []Expr }
type Bytes struct{ S []byte }
type NameConstant struct{ Value Singleton }
type Ellipsis struct{}
type ConstantExpr struct{ Value interface{} }

// the following expression can appear in assignment context
type Attribute struct {
	Value Expr
	Attr  Identifier
	Ctx   ExprContext
}
type Subscript struct {
	Value Expr
	Slice Slice
	Ctx   ExprContext
}
type Starred struct {
	Value Expr
	Ctx   ExprContext
}
type Name struct {
	Id  Identifier
	Ctx ExprContext
}
type List struct {
	Elts []Expr
	Ctx  ExprContext
}
type Tuple struct {
	Elts []Expr
	Ctx  ExprContext
}

type ExprContext int

const (
	LoadStore ExprContext = iota
	Del
	AugLoad
	AugStore
	Param
)

type Slice interface {
	slice()
}

type RangeSlice struct {
	Lower Expr
	Upper Expr
	Step  Expr
}
type ExtSlice struct{ Dims *Slice }
type Index struct{ Value Expr }

func (RangeSlice) slice() {}
func (ExtSlice) slice()   {}
func (Index) slice()      {}

type BoolOp int

const (
	And BoolOp = iota
	Or
)

type Operator int

const (
	Add Operator = iota
	Sub
	Mult
	MatMult
	Div
	Mod
	Pow
	LShift

	RShift
	BitOr
	BitXor
	BitAnd
	FloorDiv
)

type UnaryOp int

const (
	Invert UnaryOp = iota
	Not
	UAdd
	USub
)

type CmpOp int

const (
	Eq CmpOp = iota
	NotEq
	Lt
	LtE
	Gt
	GtE
	Is
	IsNot
	In
	NotIn
)

type Comprehension struct {
	Target  Expr
	Iter    Expr
	Ifs     []Expr
	IsAsync int
}

type ExceptHandler struct {
	Typ  Expr
	Name Identifier
	Body []Stmt
}

type Arguments struct {
	Args       []Arg
	Vararg     *Arg
	Kwonlyargs []Arg
	KwDefaults []Expr
	Kwarg      *Arg
	Defaults   []Expr
}

type Arg struct {
	Arg        Identifier
	Annotation Expr
}

// keyword arguments supplied to call (NULL identifier for **kwargs)
type Keyword struct {
	Arg   *Identifier
	Value Expr
}

// import name with optional 'as' alias.
type Alias struct {
	Name   Identifier
	Asname *Identifier
}

type WithItem struct {
	ContextExpr  Expr
	OptionalVars Expr
}

type Identifier string

type Singleton int

const (
	None Singleton = iota
	True
	False
)
