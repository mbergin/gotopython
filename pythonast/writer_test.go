package pythonast

import (
	"bytes"
	"testing"
)

var (
	a = &Name{Id: Identifier("a")}
	b = &Name{Id: Identifier("b")}
	c = &Name{Id: Identifier("c")}
	d = &Name{Id: Identifier("d")}
)

func bin(a Expr, op Operator, b Expr) Expr {
	return &BinOp{Left: a, Op: op, Right: b}
}

func call(f Expr, args ...Expr) Expr {
	return &Call{Func: f, Args: args}
}

func attr(e Expr, attr *Name) Expr {
	return &Attribute{Value: e, Attr: attr.Id}
}

func tup(e ...Expr) Expr {
	return &Tuple{Elts: e}
}

func eq(e1, e2 Expr) Expr {
	return &Compare{Left: e1, Ops: []CmpOp{Eq}, Comparators: []Expr{e2}}
}

func args(names ...*Name) Arguments {
	args := make([]Arg, len(names))
	for i := range names {
		args[i] = Arg{Arg: names[i].Id}
	}
	return Arguments{Args: args}
}

func lambda(args Arguments, body Expr) Expr {
	return &Lambda{Args: args, Body: body}
}

func star(e Expr) Expr {
	return &Starred{Value: e}
}

func TestExpr(t *testing.T) {
	tests := []struct {
		expr Expr
		want string
	}{
		{a, "a"},
		{bin(a, Add, b), "a + b"},
		{bin(bin(a, Sub, b), Sub, c), "a - b - c"},
		{bin(bin(bin(a, Sub, b), Sub, c), Sub, d), "a - b - c - d"},
		{bin(a, Sub, bin(b, Sub, c)), "a - (b - c)"},
		{bin(a, Add, bin(b, Mult, c)), "a + b * c"},
		{bin(a, Mult, bin(b, Add, c)), "a * (b + c)"},
		{bin(a, Pow, bin(b, Pow, c)), "a ** b ** c"},
		{bin(bin(a, Pow, b), Pow, c), "(a ** b) ** c"},
		{call(a, b, c), "a(b, c)"},
		{call(a, attr(b, c)), "a(b.c)"},
		{call(a, tup(b, c)), "a((b, c))"},
		{tup(), "()"},
		{tup(a), "a,"},
		{tup(a, b), "a, b"},
		{tup(a, tup(b, c)), "a, (b, c)"},
		{tup(a, attr(b, c)), "a, b.c"},
		{attr(tup(a, b), c), "(a, b).c"},
		{eq(tup(a, b), tup(c, d)), "(a, b) == (c, d)"},
		{tup(a, eq(b, c), d), "a, b == c, d"},
		{tup(lambda(args(a), b), c), "lambda a: b, c"},
		{lambda(args(a), tup(b, c)), "lambda a: (b, c)"},
		{call(a, star(b)), "a(*b)"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf)
			w.WriteExpr(test.expr)
			got := buf.String()
			if test.want != got {
				t.Errorf("want %q got %q", test.want, got)
			}
		})
	}
}
