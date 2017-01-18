package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
)

// Python scope
type scope struct {
	parent *scope
	ids    map[*ast.Ident]py.Identifier
	locals map[py.Identifier]bool
}

func newScope() *scope {
	return &scope{
		ids:    make(map[*ast.Ident]py.Identifier),
		locals: make(map[py.Identifier]bool),
	}
}

func (s *scope) nested() *scope {
	return &scope{
		parent: s,
		ids:    make(map[*ast.Ident]py.Identifier),
		locals: make(map[py.Identifier]bool),
	}
}

func (s *scope) id(goID *ast.Ident) py.Identifier {
	if id, ok := s.ids[goID]; ok {
		return id
	}
	pyID := py.Identifier(goID.Name)
	for i := 0; s.locals[pyID]; i++ {
		pyID = py.Identifier(fmt.Sprintf("%s%d", goID.Name, i))
	}
	s.ids[goID] = pyID
	s.locals[pyID] = true
	return pyID
}

func (s *scope) tempID(baseId string) py.Identifier {
	pyID := py.Identifier(baseId)
	for i := 0; s.locals[pyID]; i++ {
		pyID = py.Identifier(fmt.Sprintf("%s%d", baseId, i))
	}
	s.locals[pyID] = true
	return pyID
}
