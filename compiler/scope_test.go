package compiler

import (
	py "github.com/mbergin/gotopython/pythonast"
	"go/token"
	"go/types"
	"testing"
)

func Test_scope_id_different_idents(t *testing.T) {
	scope := newScope()
	x1 := scope.id(types.NewVar(token.NoPos, nil, "x", nil))
	x2 := scope.id(types.NewVar(token.NoPos, nil, "x", nil))
	if x1 != py.Identifier("x") {
		t.Errorf("x1=", x1)
	}
	if x2 != py.Identifier("x1") {
		t.Errorf("x2=", x2)
	}
}

func Test_scope_id_same_idents(t *testing.T) {
	scope := newScope()
	x := types.NewVar(token.NoPos, nil, "x", nil)
	x1 := scope.id(x)
	x2 := scope.id(x)
	if x1 != py.Identifier("x") {
		t.Errorf("x1=", x1)
	}
	if x2 != py.Identifier("x") {
		t.Errorf("x2=", x2)
	}
}
