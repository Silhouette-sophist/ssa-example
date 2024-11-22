package service

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"log"
	"os"
	"path/filepath"
	"ssa-example/visitor"
	"strings"
)

// CallGraph 获取调用图
func CallGraph(rootDir string) (*callgraph.Graph, error) {
	// -1.校验参数
	if rootDir == "" {
		return nil, errors.New("invalid param")
	}
	modFilePath, err := ExecGoCommandWithDir(rootDir, []string{"env", "GOMOD"})
	if err != nil {
		return nil, err
	}
	fmt.Printf("mod file path %s\n", modFilePath)
	data, err := os.ReadFile(modFilePath)
	if err != nil {
		log.Fatalf("Failed to read go.mod: %v", err)
		return nil, err
	}
	modFile, err := modfile.Parse(modFilePath, data, nil)
	if err != nil {
		log.Fatalf("Failed to parse go.mod: %v", err)
		return nil, err
	}
	// 匹配模块名&匹配依赖项
	fmt.Println("Module Name:", modFile.Module.Mod.Path)
	fmt.Println("Dependencies:")
	for _, dep := range modFile.Require {
		fmt.Printf(" - %s %s\n", dep.Mod.Path, dep.Mod.Version)
	}
	rootPkg := modFile.Module.Mod.Path
	// 1.配置
	config := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: false,
		Dir:   rootDir,
	}
	// 2.加载packages包
	loadPackages, err := packages.Load(config)
	if err != nil {
		fmt.Printf("loadPackages error %v\n", err)
		return nil, err
	}
	fmt.Printf("loadPackages %v\n", loadPackages)
	// 3.加载所有ssa包和ssa程序
	ssaProgram, ssaPackages := ssautil.AllPackages(loadPackages, 0)
	fmt.Printf("ssaProgram %v, ssaPackages %v\n", ssaProgram, ssaPackages)
	// 4.构建ssa程序
	ssaProgram.Build()
	// 5.找main包
	mainPackages := ssautil.MainPackages(ssaPackages)
	fmt.Printf("mainPackages %v\n", mainPackages)
	// 6.查找已经被导入的包信息
	usedFuncMap := getRepoUsedFuncMap(loadPackages, rootPkg, rootDir)
	// 7.创建Ssa调用图
	g, err := CreateSsaCallGraph("vta", ssaProgram, mainPackages, usedFuncMap)
	if err != nil {
		fmt.Printf("createSsaCallGraph error: %v", err)
		return nil, err
	}
	return g, err
}

// CreateSsaCallGraph 创建ssa调用图
func CreateSsaCallGraph(algo string, prog *ssa.Program, mainPackages []*ssa.Package, usedFuncMap map[string]struct{}) (*callgraph.Graph, error) {
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
		for function := range allFunctions {
			if _, ok := usedFuncMap[function.Name()]; !ok {
				allFunctions[function] = false
			}
		}
		return vta.CallGraph(allFunctions, cha.CallGraph(prog)), nil
	}
	return nil, errors.New("invalid flow")
}

func getRepoUsedFuncMap(pkgs []*packages.Package, rootPkg, rootDir string) map[string]struct{} {
	usedMap := make(map[string]struct{})
	viewedMap := make(map[string]struct{})
	rootDir = getAbsPath(rootDir)
	for _, pkg := range pkgs {
		if !isCurrentRepoFunc(pkg.PkgPath, rootPkg) {
			continue
		}
		// packages.Package.Fset 字段是一个 *token.FileSet 类型，它用于跟踪源文件中的位置信息。
		// 具体来说，Fset 包含了通过 packages 包加载的所有文件的信息，但它并不直接包含所有使用到的文件的集合。
		pkg.Fset.Iterate(func(file *token.File) bool {
			fmt.Printf("-------pkg %s, %s file %s\n", pkg.Name, pkg.PkgPath, file.Name())
			if !strings.HasPrefix(file.Name(), rootDir) {
				return true
			}
			if _, ok := viewedMap[file.Name()]; ok {
				return true
			}
			viewedMap[file.Name()] = struct{}{}
			fileSet := token.NewFileSet()
			readFile, err := os.ReadFile(file.Name())
			if err != nil {
				return true
			}
			parse, err := parser.ParseFile(fileSet, file.Name(), readFile, parser.ParseComments)
			if err != nil {
				return true
			}
			ast.Walk(&visitor.FuncUsedScanner{
				IdentMap: usedMap,
			}, parse)
			return true
		})
	}
	return usedMap
}

func isCurrentRepoFunc(pkgPath, rootPkg string) bool {
	return pkgPath == rootPkg || strings.HasPrefix(pkgPath, rootPkg+"/") ||
		strings.HasPrefix(pkgPath, rootPkg+"(") || strings.HasPrefix(pkgPath, rootPkg+"#")
}

func getAbsPath(filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	abs, _ := filepath.Abs(filePath)
	return abs
}
