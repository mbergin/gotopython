package compiler

import (
	"github.com/davecgh/go-spew/spew"
	py "github.com/mbergin/gotopython/pythonast"
	"go/parser"
	"reflect"
	"testing"
)

// Placeholder expressions used in expr tests
var (
	w = &py.Name{Id: py.Identifier("w")}
	x = &py.Name{Id: py.Identifier("x")}
	y = &py.Name{Id: py.Identifier("y")}
	z = &py.Name{Id: py.Identifier("z")}
	// Type expressions
	T = &py.Name{Id: py.Identifier("T")}
	U = &py.Name{Id: py.Identifier("U")}
)

var exprTests = []struct {
	golang string
	python py.Expr
}{
	// Identifier
	{"myVar", &py.Name{Id: py.Identifier("myVar")}},

	// Predeclared identifiers
	{"true", &py.NameConstant{Value: py.True}},
	{"false", &py.NameConstant{Value: py.False}},
	{"nil", &py.NameConstant{Value: py.None}},

	// Integer literals
	{"42", &py.Num{N: "42"}},
	//{"0600", &py.Num{N: "0o600"}},
	{"0xBadFace", &py.Num{N: "0xBadFace"}},
	{"170141183460469231731687303715884105727", &py.Num{N: "170141183460469231731687303715884105727"}},

	// Floating point literals
	{"0.", &py.Num{N: "0."}},
	{"72.40", &py.Num{N: "72.40"}},
	{"072.40", &py.Num{N: "072.40"}},
	{"2.71828", &py.Num{N: "2.71828"}},
	{"1.e+0", &py.Num{N: "1.e+0"}},
	{"6.67428e-11", &py.Num{N: "6.67428e-11"}},
	{"1E6", &py.Num{N: "1E6"}},
	{".25", &py.Num{N: ".25"}},
	{".12345E+5", &py.Num{N: ".12345E+5"}},

	// Imaginary literals
	// {"0i", &py.Num{N: "0j"}},
	// {"011i", &py.Num{N: "011j"}},
	// {"0.i", &py.Num{N: "0.j"}},
	// {"2.71828i", &py.Num{N: "2.71828j"}},
	// {"1.e+0i", &py.Num{N: "1.e+0j"}},
	// {"6.67428e-11i", &py.Num{N: "6.67428e-11j"}},
	// {"1E6i", &py.Num{N: "1E6j"}},
	// {".25i", &py.Num{N: ".25j"}},
	// {".12345E+5i", &py.Num{N: ".12345E+5j"}},

	// String literals
	{`""`, &py.Str{S: `""`}},
	{`"hello world"`, &py.Str{S: `"hello world"`}},
	{`"a"`, &py.Str{S: `"a"`}},
	{`"ä"`, &py.Str{S: `"ä"`}},
	{`"本"`, &py.Str{S: `"本"`}},
	{`"\t"`, &py.Str{S: `"\t"`}},
	{`"\000"`, &py.Str{S: `"\000"`}},
	{`"\007"`, &py.Str{S: `"\007"`}},
	{`"\377"`, &py.Str{S: `"\377"`}},
	{`"\x07"`, &py.Str{S: `"\x07"`}},
	{`"\xff"`, &py.Str{S: `"\xff"`}},
	{`"\u12e4"`, &py.Str{S: `"\u12e4"`}},
	{`"\U00101234"`, &py.Str{S: `"\U00101234"`}},
	{`"\""`, &py.Str{S: `"\""`}},

	// Rune literals
	{`'a'`, &py.Str{S: `'a'`}},
	{`'ä'`, &py.Str{S: `'ä'`}},
	{`'本'`, &py.Str{S: `'本'`}},
	{`'\t'`, &py.Str{S: `'\t'`}},
	{`'\000'`, &py.Str{S: `'\000'`}},
	{`'\007'`, &py.Str{S: `'\007'`}},
	{`'\377'`, &py.Str{S: `'\377'`}},
	{`'\x07'`, &py.Str{S: `'\x07'`}},
	{`'\xff'`, &py.Str{S: `'\xff'`}},
	{`'\u12e4'`, &py.Str{S: `'\u12e4'`}},
	{`'\U00101234'`, &py.Str{S: `'\U00101234'`}},
	{`'\''`, &py.Str{S: `'\''`}},

	// Composite literals
	{"T{}", &py.Call{Func: T}},
	{"T{x, y}", &py.Call{Func: T, Args: []py.Expr{x, y}}},
	{"T{x: y}", &py.Call{Func: T, Keywords: []py.Keyword{py.Keyword{Arg: &x.Id, Value: y}}}},
	{"[2]T{x, y}", &py.List{Elts: []py.Expr{x, y}}},
	{"[...]T{x, y}", &py.List{Elts: []py.Expr{x, y}}},
	{"[]T{x, y}", &py.List{Elts: []py.Expr{x, y}}},
	{"[]T{x, y}", &py.List{Elts: []py.Expr{x, y}}},
	{"map[T]U{}", &py.Dict{
		Keys:   []py.Expr{},
		Values: []py.Expr{},
	}},
	{"map[T]U{x: y, z: w}", &py.Dict{
		Keys:   []py.Expr{x, z},
		Values: []py.Expr{y, w},
	}},
	{"[]T{{x},{y}}", &py.List{
		Elts: []py.Expr{
			&py.Call{Func: T, Args: []py.Expr{x}},
			&py.Call{Func: T, Args: []py.Expr{y}},
		},
	}},
	{"map[T]U{{x}: {y}, {z}: {w}}", &py.Dict{
		Keys: []py.Expr{
			&py.Call{Func: T, Args: []py.Expr{x}},
			&py.Call{Func: T, Args: []py.Expr{z}},
		},
		Values: []py.Expr{
			&py.Call{Func: U, Args: []py.Expr{y}},
			&py.Call{Func: U, Args: []py.Expr{w}},
		},
	}},

	// Comparison operators
	{"x==y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Eq}}},
	{"x!=y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.NotEq}}},
	{"x< y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Lt}}},
	{"x<=y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.LtE}}},
	{"x> y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Gt}}},
	{"x>=y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.GtE}}},

	// Arithmetic operators
	{"x+ y", &py.BinOp{Left: x, Right: y, Op: py.Add}},
	{"x- y", &py.BinOp{Left: x, Right: y, Op: py.Sub}},
	{"x* y", &py.BinOp{Left: x, Right: y, Op: py.Mult}},
	{"x/ y", &py.BinOp{Left: x, Right: y, Op: py.FloorDiv}}, //TODO FloorDiv if ints, Div if floats
	{"x% y", &py.BinOp{Left: x, Right: y, Op: py.Mod}},
	{"x& y", &py.BinOp{Left: x, Right: y, Op: py.BitAnd}},
	{"x| y", &py.BinOp{Left: x, Right: y, Op: py.BitOr}},
	{"x^ y", &py.BinOp{Left: x, Right: y, Op: py.BitXor}},
	{"x<<y", &py.BinOp{Left: x, Right: y, Op: py.LShift}},
	{"x>>y", &py.BinOp{Left: x, Right: y, Op: py.RShift}},
	{"x&^y", &py.BinOp{Left: x, Right: &py.UnaryOpExpr{Operand: y, Op: py.Invert}, Op: py.BitAnd}},

	// Logical operators
	{"x&&y", &py.BoolOpExpr{Values: []py.Expr{x, y}, Op: py.And}},
	{"x||y", &py.BoolOpExpr{Values: []py.Expr{x, y}, Op: py.Or}},
	{"!x", &py.UnaryOpExpr{Operand: x, Op: py.Not}},

	// Address operators
	{"&x", x},
	// {"*x", y},

	// Parenthesis
	{"(x)", x},

	// Unary operators
	{"-x", &py.UnaryOpExpr{Operand: x, Op: py.USub}},
	{"+x", &py.UnaryOpExpr{Operand: x, Op: py.UAdd}},
	{"^x", &py.UnaryOpExpr{Operand: x, Op: py.Invert}}, // TODO incorrect for unsigned

	// Selector
	{"x.y", &py.Attribute{Value: x, Attr: y.Id}},

	// Call
	{"x()", &py.Call{Func: x}},
	{"x(y)", &py.Call{Func: x, Args: []py.Expr{y}}},
	{"x(y,z)", &py.Call{Func: x, Args: []py.Expr{y, z}}},
	//{"x(y,z...)", &py.Call{Func: x, Args: []py.Expr{y, &py.Starred{Value: z}}}},

	// Index
	{"x[y]", &py.Subscript{Value: x, Slice: &py.Index{Value: y}}},

	// Slice
	{"x[y:z]", &py.Subscript{Value: x, Slice: &py.RangeSlice{Lower: y, Upper: z}}},
	{"x[y:]", &py.Subscript{Value: x, Slice: &py.RangeSlice{Lower: y}}},
	{"x[:z]", &py.Subscript{Value: x, Slice: &py.RangeSlice{Upper: z}}},
	{"x[:]", &py.Subscript{Value: x, Slice: &py.RangeSlice{}}},

	// Built-in functions
	{"make([]T, x)", &py.ListComp{
		Elt: &py.Call{Func: T},
		Generators: []py.Comprehension{
			py.Comprehension{
				Target: &py.Name{Id: py.Identifier("_")},
				Iter: &py.Call{
					Func: pyRange,
					Args: []py.Expr{x}},
			}}}},
	{"make([]T, x, y)", &py.ListComp{
		Elt: &py.Call{Func: T},
		Generators: []py.Comprehension{
			py.Comprehension{
				Target: &py.Name{Id: py.Identifier("_")},
				Iter: &py.Call{
					Func: pyRange,
					Args: []py.Expr{x}},
			}}}},
	{"make(map[T]U)", &py.Dict{}},
	// TODO this should return a byte count for strings --
	// len() in Python returns a character count
	{"len(x)", &py.Call{Func: pyLen, Args: []py.Expr{x}}},
}

var sp = spew.NewDefaultConfig()

func init() {
	sp.DisablePointerAddresses = true
}

func TestExpr(t *testing.T) {

	for _, test := range exprTests {
		goExpr, err := parser.ParseExpr(test.golang)
		if err != nil {
			t.Errorf("failed to parse Go expr %q: %s", test.golang, err)
			continue
		}
		pyExpr := compileExpr(goExpr)
		if !reflect.DeepEqual(pyExpr, test.python) {
			t.Errorf("want \n%s got \n%s", sp.Sdump(test.python), sp.Sdump(pyExpr))
		}
	}
}
