package main

import (
	"flag"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/mbergin/gotopython/compiler"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

var (
	dumpGoAST     = flag.Bool("g", false, "Dump the Go syntax tree to stdout")
	dumpPythonAST = flag.Bool("p", false, "Dump the Python syntax tree to stdout")
	output        = flag.String("o", "", "Write the Python module to this file")
	httpAddress   = flag.String("http", "", "HTTP service address (e.g. ':8080')")
)

var (
	errInput  = 1
	errOutput = 2
	errNoDir  = 3
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gotopython [flags] packagedir\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *httpAddress != "" {
		runWebServer(*httpAddress)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(errNoDir)
	}

	dir := flag.Arg(0)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(errInput)
	}

	if *dumpGoAST {
		ast.Print(fset, pkgs)
	}

	for _, pkg := range pkgs {
		module := compiler.CompilePackage(pkg)
		if *dumpPythonAST {
			spew.Dump(module)
		}
		writer := os.Stdout
		if *output != "" {
			writer, err = os.Create(*output)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(errOutput)
			}
		}
		pyWriter := py.NewWriter(writer)
		pyWriter.WriteModule(module)
	}
}
