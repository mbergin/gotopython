package compiler

import (
	"bytes"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/loader"
	"reflect"
	"testing"
)

// Each test compiles this code with the expression under test substituted for %s
const exprPkgTemplate = `package main

type T struct{x, y int}
var (
	t0 = T{}
	t1 = T{}
)

type U struct{}

var (
	b0, b1 bool
	w, x, y, z int
	u0, u1 uint
	xs []int
	obj interface{}
)

func f0() int { return 0 }
func f1(int) int { return 0 }
func f2(int, int) int { return 0 }

var expr = %s
`

// These placeholder expressions match the names in the above code
var (
	f0 = &py.Name{Id: py.Identifier("f0")}
	f1 = &py.Name{Id: py.Identifier("f1")}
	f2 = &py.Name{Id: py.Identifier("f2")}

	b0 = &py.Name{Id: py.Identifier("b0")}
	b1 = &py.Name{Id: py.Identifier("b1")}

	u0 = &py.Name{Id: py.Identifier("u0")}
	u1 = &py.Name{Id: py.Identifier("u1")}

	w = &py.Name{Id: py.Identifier("w")}
	x = &py.Name{Id: py.Identifier("x")}
	y = &py.Name{Id: py.Identifier("y")}
	z = &py.Name{Id: py.Identifier("z")}

	xs = &py.Name{Id: py.Identifier("xs")}

	T  = &py.Name{Id: py.Identifier("T")}
	t0 = &py.Name{Id: py.Identifier("t0")}
	t1 = &py.Name{Id: py.Identifier("t1")}

	U = &py.Name{Id: py.Identifier("U")}

	obj = &py.Name{Id: py.Identifier("obj")}
	m   = &py.Name{Id: py.Identifier("m")}
)

var exprTests = []struct {
	golang string
	python py.Expr
}{
	// Identifier
	{"x", x},

	// Predeclared identifiers
	{"true", &py.NameConstant{Value: py.True}},
	{"false", &py.NameConstant{Value: py.False}},
	//{"(*T)(nil)", &py.NameConstant{Value: py.None}},

	// Integer literals
	{"42", &py.Num{N: "42"}},
	//{"0600", &py.Num{N: "0o600"}},
	{"0xBadFace", &py.Num{N: "0xBadFace"}},
	//{"170141183460469231731687303715884105727", &py.Num{N: "170141183460469231731687303715884105727"}},

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
	{"0i", &py.Num{N: "0j"}},
	{"011i", &py.Num{N: "011j"}},
	{"0.i", &py.Num{N: "0.j"}},
	{"2.71828i", &py.Num{N: "2.71828j"}},
	{"1.e+0i", &py.Num{N: "1.e+0j"}},
	{"6.67428e-11i", &py.Num{N: "6.67428e-11j"}},
	{"1E6i", &py.Num{N: "1E6j"}},
	{".25i", &py.Num{N: ".25j"}},
	{".12345E+5i", &py.Num{N: ".12345E+5j"}},

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
	{"[2]T{t0, t1}", &py.List{Elts: []py.Expr{t0, t1}}},
	{"[...]T{t0, t1}", &py.List{Elts: []py.Expr{t0, t1}}},
	{"[]T{t0, t1}", &py.List{Elts: []py.Expr{t0, t1}}},
	{"map[T]U{}", &py.Dict{
		Keys:   []py.Expr{},
		Values: []py.Expr{},
	}},
	{"map[int]int{x: y, z: w}", &py.Dict{
		Keys:   []py.Expr{x, z},
		Values: []py.Expr{y, w},
	}},
	{"[]T{{x, y},{z, w}}", &py.List{
		Elts: []py.Expr{
			&py.Call{Func: T, Args: []py.Expr{x, y}},
			&py.Call{Func: T, Args: []py.Expr{z, w}},
		},
	}},
	{"map[T]U{{x, y}: {}, {z, w}: {}}", &py.Dict{
		Keys: []py.Expr{
			&py.Call{Func: T, Args: []py.Expr{x, y}},
			&py.Call{Func: T, Args: []py.Expr{z, w}},
		},
		Values: []py.Expr{
			&py.Call{Func: U},
			&py.Call{Func: U},
		},
	}},

	// Comparison operators
	{"x == y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Eq}}},
	{"x != y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.NotEq}}},
	{"x < y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Lt}}},
	{"x <= y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.LtE}}},
	{"x > y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.Gt}}},
	{"x >= y", &py.Compare{Left: x, Comparators: []py.Expr{y}, Ops: []py.CmpOp{py.GtE}}},

	// Arithmetic operators
	{"x + y", &py.BinOp{Left: x, Right: y, Op: py.Add}},
	{"x - y", &py.BinOp{Left: x, Right: y, Op: py.Sub}},
	{"x * y", &py.BinOp{Left: x, Right: y, Op: py.Mult}},
	{"x / y", &py.BinOp{Left: x, Right: y, Op: py.FloorDiv}}, //TODO FloorDiv if ints, Div if floats
	{"x % y", &py.BinOp{Left: x, Right: y, Op: py.Mod}},
	{"x & y", &py.BinOp{Left: x, Right: y, Op: py.BitAnd}},
	{"x | y", &py.BinOp{Left: x, Right: y, Op: py.BitOr}},
	{"x ^ y", &py.BinOp{Left: x, Right: y, Op: py.BitXor}},
	{"x << u0", &py.BinOp{Left: x, Right: u0, Op: py.LShift}},
	{"x >> u0", &py.BinOp{Left: x, Right: u0, Op: py.RShift}},
	{"x &^ y", &py.BinOp{Left: x, Right: &py.UnaryOpExpr{Operand: y, Op: py.Invert}, Op: py.BitAnd}},

	// Logical operators
	{"b0 && b1", &py.BoolOpExpr{Values: []py.Expr{b0, b1}, Op: py.And}},
	{"b0 || b1", &py.BoolOpExpr{Values: []py.Expr{b0, b1}, Op: py.Or}},
	{"!b0", &py.UnaryOpExpr{Operand: b0, Op: py.Not}},

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
	{"T{}.y", &py.Attribute{
		Value: &py.Call{
			Func: T,
		},
		Attr: y.Id,
	}},

	// Call
	{"f0()", &py.Call{Func: f0}},
	{"f1(y)", &py.Call{Func: f1, Args: []py.Expr{y}}},
	{"f2(y,z)", &py.Call{Func: f2, Args: []py.Expr{y, z}}},
	//{"x(y,z...)", &py.Call{Func: x, Args: []py.Expr{y, &py.Starred{Value: z}}}},

	// Index
	{"xs[y]", &py.Subscript{Value: xs, Slice: &py.Index{Value: y}}},

	// Slice
	{"xs[y:z]", &py.Subscript{Value: xs, Slice: &py.RangeSlice{Lower: y, Upper: z}}},
	{"xs[y:]", &py.Subscript{Value: xs, Slice: &py.RangeSlice{Lower: y}}},
	{"xs[:z]", &py.Subscript{Value: xs, Slice: &py.RangeSlice{Upper: z}}},
	{"xs[:]", &py.Subscript{Value: xs, Slice: &py.RangeSlice{}}},

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
	{"len(xs)", &py.Call{Func: pyLen, Args: []py.Expr{xs}}},
	{`len("")`, &py.Call{
		Func: pyLen,
		Args: []py.Expr{
			&py.Call{
				Func: &py.Attribute{Value: pyEmptyString, Attr: py.Identifier("encode")},
				Args: []py.Expr{&py.Str{S: `"utf-8"`}},
			},
		},
	}},
	{"cap(xs)", &py.Call{Func: pyLen, Args: []py.Expr{xs}}},
	{"new(T)", &py.Call{Func: T}},
	{"new(int)", &py.Num{N: "0"}},
	{"complex(1.0, 2.0)", &py.Call{Func: pyComplex, Args: []py.Expr{&py.Num{N: "1.0"}, &py.Num{N: "2.0"}}}},
	{"real(1+2i)", &py.Attribute{
		Attr:  py.Identifier("real"),
		Value: &py.BinOp{Left: &py.Num{N: "1"}, Op: py.Add, Right: &py.Num{N: "2j"}}}},
	{"imag(1+2i)", &py.Attribute{
		Attr:  py.Identifier("imag"),
		Value: &py.BinOp{Left: &py.Num{N: "1"}, Op: py.Add, Right: &py.Num{N: "2j"}}}},
}

var sp = spew.NewDefaultConfig()

func init() {
	sp.DisablePointerAddresses = true
}

func pythonExprCode(expr py.Expr) string {
	var buf bytes.Buffer
	writer := py.NewWriter(&buf)
	writer.WriteExpr(expr)
	return buf.String()
}

func buildFile(file string) (*loader.PackageInfo, *ast.File, []error) {
	var conf loader.Config
	conf.AllowErrors = true
	conf.Fset = token.NewFileSet()
	astFile, err := parser.ParseFile(conf.Fset, "main.go", file, parser.ParseComments)
	if err != nil {
		return nil, nil, []error{err}
	}

	conf.CreateFromFiles("main", astFile)
	program, err := conf.Load()
	if err != nil {
		return nil, nil, []error{err}
	}
	pkg := program.Package("main")
	return pkg, pkg.Files[0], pkg.Errors
}

func TestExpr(t *testing.T) {
	for _, test := range exprTests {
		pkg, file, errs := buildFile(fmt.Sprintf(exprPkgTemplate, test.golang))
		if errs != nil {
			t.Errorf("failed to build Go expr %q", test.golang)
			for e := range errs {
				t.Error(e)
			}
			continue
		}

		c := NewCompiler(pkg.Info)
		goExpr := file.Scope.Lookup("expr").Decl.(*ast.ValueSpec).Values[0]
		pyExpr := c.compileExpr(goExpr)
		if !reflect.DeepEqual(pyExpr, test.python) {
			t.Errorf("\nwant %s\ngot  %s", pythonExprCode(test.python), pythonExprCode(pyExpr))
			spew.Dump(test.python)
			spew.Dump(pyExpr)
		}
	}
}
