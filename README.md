A work-in-progress Go to Python transpiler. 
It can currently only compile toy programs.

# Example usage

This will compile the package at path `./mypackage` into a Python module `mypackage.py`:

```
gotopython -o mypackage.py ./mypackage
```

# Implementation status

The parts of the Go language spec that are implemented are:

| Expression     | Example                   | Implemented |
|----------------|---------------------------|-------------|
| BadExpr        |                           | n/a         |
| Ident          | `myVar`                   | ✓           |
| Ellipsis       | `...`                     |             |
| BasicLit       | `42`                      | ✓           |
| FuncLit        | `func(t T) {}`            |             |
| CompositeLit   | `T{x: 1, y: 2}`           | ✓           |
| ParenExpr      | `(x)`                     | ✓           |
| SelectorExpr   | `x.y`                     | ✓           |
| IndexExpr      | `x[y]`                    | ✓           |
| SliceExpr      | `x[y:z]`                  | ✓           |
| TypeAssertExpr | `x.(T)`                   |             |
| CallExpr       | `x(y,z)`                  | ✓           |
| StarExpr       | `*x`                      |             |
| UnaryExpr      | `-x`                      | ✓           |
| BinaryExpr     | `x+y`                     | ✓           |
| KeyValueExpr   | `x: y`                    | ✓           |
| ArrayType      | `[]T`                     | ✓           |
| StructType     | `struct { T x }`          |             |
| FuncType       | `func(T) U`               |             |
| InterfaceType  | `interface {}`            |             |
| MapType        | `map[T]U`                 |             |
| ChanType       | `chan<- T`                |             |

| Statement      | Example                     | Implemented |
|----------------|-----------------------------|-------------|
| BadStmt        |                             | n/a         |
| DeclStmt       | `var x T`                   |             |
| EmptyStmt      |                             |             |
| LabeledStmt    | `label: ...`                |             |
| ExprStmt       | `x`                         | ✓           |
| SendStmt       | `x <- y`                    |             |
| IncDecStmt     | `x++`                       | ✓           |
| AssignStmt     | `x, y := z`                 | ✓           |
| GoStmt         | `go f()`                    |             |
| DeferStmt      | `defer f()`                 |             |
| ReturnStmt     | `return x, y`               | 1           |
| BranchStmt     | `break`                     | ✓           |
| BlockStmt      | `{...}`                     | ✓           |
| IfStmt         | `if x; y {...}`             | ✓           |
| CaseClause     | `case x>y:`                 | ✓           |
| SwitchStmt     | `switch x; y {...}`         | 2           |
| TypeSwitchStmt | `switch x.(type) {...}`     |             | 
| CommClause     | `case x = <-y: ...`         |             |
| SelectStmt     | `select { ... }`            |             |
| ForStmt        | `for x; y; z {...}`         | ✓           |
| RangeStmt      | `for x, y := range z {...}` | 3           |

1. No argumentless return in functions with named return values
2. No `fallthrough`
3. Only for array/slice

| Spec       | Example      | Implemented |
|------------|--------------|-------------|
| ImportSpec | `import "x"` |             |
| ValueSpec  | `var x T`    |             |
| TypeSpec   | `type T U`   | ✓           |

| Built-in function | Implemented |
|-------------------| ------------|
| `close`           |             |
| `len`             |             |
| `cap`             |             |
| `new`             |             |
| `make([]T)`       | ✓           |
| `make(map[T]U)`   |             |
| `make(chan T)`    |             |
| `append`          |             |
| `copy`            |             |
| `delete`          |             |
| `complex`         |             |
| `real`            |             |
| `imag`            |             |
| `panic`           |             |
| `recover`         |             |
| `print`           |             |
| `println`         |             |

| Language feature     | Implemented |
|----------------------|-------------|
| fixed width integers |             |
| struct copying       |             |
| pass by value        |             |
| package unsafe       |             |
| goroutines           |             |
| Imports              |             |
| Name collisions      |             |
| Scoping rules        |             |
| `fallthrough`        |             |
| `goto`               |             |
| cgo                  |             |
