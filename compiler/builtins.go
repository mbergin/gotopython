package compiler

import py "github.com/mbergin/gotopython/pythonast"

var (
	pyTrue        = &py.NameConstant{Value: py.True}
	pyFalse       = &py.NameConstant{Value: py.False}
	pyNone        = &py.NameConstant{Value: py.None}
	pyEmptyString = &py.Str{S: `""`}
	pyRange       = &py.Name{Id: py.Identifier("range")}
	pyLen         = &py.Name{Id: py.Identifier("len")}
	pyEnumerate   = &py.Name{Id: py.Identifier("enumerate")}
	pyType        = &py.Name{Id: py.Identifier("type")}
	pyKeyError    = &py.Name{Id: py.Identifier("KeyError")}
	pyComplex     = &py.Name{Id: py.Identifier("complex")}
)
