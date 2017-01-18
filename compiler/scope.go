package compiler

import (
	"fmt"
	py "github.com/mbergin/gotopython/pythonast"
	"go/types"
)

// Python scope
type scope struct {
	parent *scope
	ids    map[types.Object]py.Identifier
	locals map[py.Identifier]bool
}

func newScope() *scope {
	return &scope{
		ids:    make(map[types.Object]py.Identifier),
		locals: make(map[py.Identifier]bool),
	}
}

func (s *scope) nested() *scope {
	ns := newScope()
	ns.parent = s
	return ns
}

func (s *scope) id(goID types.Object) py.Identifier {
	if id, ok := s.ids[goID]; ok {
		return id
	}
	pyID := py.Identifier(goID.Name())
	for i := 1; s.locals[pyID]; i++ {
		pyID = py.Identifier(fmt.Sprintf("%s%d", goID.Name(), i))
	}
	s.ids[goID] = pyID
	s.locals[pyID] = true
	return pyID
}

func (s *scope) tempID(baseId string) py.Identifier {
	pyID := py.Identifier(baseId)
	for i := 1; s.locals[pyID]; i++ {
		pyID = py.Identifier(fmt.Sprintf("%s%d", baseId, i))
	}
	s.locals[pyID] = true
	return pyID
}
