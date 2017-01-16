package main

import (
	"flag"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/mbergin/gotopython/compiler"
	py "github.com/mbergin/gotopython/pythonast"
	"go/ast"
	"go/build"
	"golang.org/x/tools/go/loader"
	"os"
)

var (
	dumpGoAST     = flag.Bool("g", false, "Dump the Go syntax tree to stdout")
	dumpPythonAST = flag.Bool("p", false, "Dump the Python syntax tree to stdout")
	output        = flag.String("o", "", "Write the Python module to this file")
	httpAddress   = flag.String("http", "", "HTTP service address (e.g. ':8080')")
)

const (
	_ = iota
	errArgs
	errParse
	errOutput
	errNoDir
	errBuild
	errTypeCheck
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gotopython [flags] package\n")
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

	var loaderConfig loader.Config
	const xtest = false
	_, err := loaderConfig.FromArgs(flag.Args(), xtest)
	// TODO ignoring args after "--"
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(errArgs)
	}

	buildContext := build.Default
	buildContext.GOARCH = "python"
	program, err := loaderConfig.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(errBuild)
	}

	for _, pkg := range program.InitialPackages() {
		if *dumpGoAST {
			spew.Dump(pkg.Info)
			for _, file := range pkg.Files {
				ast.Print(program.Fset, file)
			}
		}

		c := &compiler.Compiler{}
		module := c.CompileFiles(pkg.Files)

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
