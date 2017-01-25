package compiler

import py "github.com/mbergin/gotopython/pythonast"

var (
	runtimeModule = &py.Name{Id: py.Identifier("runtime")}
)
