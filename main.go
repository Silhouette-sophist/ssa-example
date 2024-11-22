package main

import (
	"errors"
	"fmt"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func main() {
	// 1.配置
	config := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: false,
		Dir:   "/Users/silhouette/work-practice/gin-example",
	}
	// 2.加载包
	load, err := packages.Load(config)
	if err != nil {
		fmt.Printf("load error %v\n", err)
		return
	}
	fmt.Printf("load %v\n", load)
	// 3.加载所有包
	ssaProgram, ssaPkgs := ssautil.AllPackages(load, 0)
	fmt.Printf("ssaProgram %v, ssaPkgs %v\n", ssaProgram, ssaPkgs)
	// 4.构建ssa程序
	ssaProgram.Build()
	// 5.找main包
	mainPackages := ssautil.MainPackages(ssaPkgs)
	fmt.Printf("mainPackages %v\n", mainPackages)
	// 6.创建Ssa调用图
	g, err := createSsaCallGraph("gin-example", "vta", ssaProgram, mainPackages)
	if err != nil {
		fmt.Printf("createSsaCallGraph error: %v", err)
		return
	}
	fmt.Printf("createSsaCallGraph callGraph: %v", g)
}

func createSsaCallGraph(rootPkg, algo string, prog *ssa.Program, mainPackages []*ssa.Package) (g *callgraph.Graph, err error) {
	switch algo {
	case "pta":
		pointerCfg := &pointer.Config{
			Mains:          mainPackages,
			BuildCallGraph: true,
		}
		result, err := pointer.Analyze(pointerCfg)
		if err != nil {
			return nil, err
		}
		return result.CallGraph, nil
	case "rta":
		functions := make([]*ssa.Function, 0)
		for _, m := range mainPackages {
			functions = append(functions, m.Func("init"), m.Func("main"))
		}
		return rta.Analyze(functions, true).CallGraph, nil
	case "vta":
		allFunctions := ssautil.AllFunctions(prog)
		return vta.CallGraph(allFunctions, cha.CallGraph(prog)), nil
	}
	return nil, errors.New("invalid flow")
}
