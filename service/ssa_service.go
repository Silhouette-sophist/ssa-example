package service

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
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
	"regexp"
	"ssa-example/model"
	"ssa-example/visitor"
	"strings"
)

var (
	FieldAddrReg              = regexp.MustCompile("\\[#\\d+]")
	anonymousFuncNameRegex    = regexp.MustCompile("\\$\\d+$")
	anonymousFuncLIneNumRegex = regexp.MustCompile("\\$\\d+\\(")
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

func isUsefulFunc(ssaFunc *ssa.Function, library map[string]bool) bool {
	if ssaFunc.Pkg == nil {
		return false
	}
	// 判断包是否包含
	pkgPath := ssaFunc.Pkg.Pkg.Path()
	if library[pkgPath] {
		return false
	}
	// 判断函数名
	funcName := ssaFunc.Name()
	if (funcName == "init" || strings.HasPrefix(funcName, "init#") || strings.Contains(funcName, "#init$")) && ssaFunc.Signature.Recv() == nil {
		// 过滤掉go package init子图的节点
		startPos := ssaFunc.Prog.Fset.Position(ssaFunc.Pos())
		return strings.HasSuffix(startPos.Filename, "kite.go")
	}
	if funcName == "Error" || funcName == "Close" || strings.HasPrefix(funcName, "Close$") {
		return false
	}
	return true
}

func getAbsPath(filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	abs, _ := filepath.Abs(filePath)
	return abs
}

func ConvertSsaCallGraphToCustomGraph(callGraph *callgraph.Graph, rootPkg string) (*model.CallGraph, error) {
	// 1.返回图结构
	retCallGraph := &model.CallGraph{
		NodeMap: make(map[string]*model.Node),
	}
	// 2.过滤使用到的函数map
	usedFuncMap, err := FilterUsedSsaGraphFunctions(callGraph, rootPkg)
	if err != nil {
		fmt.Printf("errror...\n")
		return nil, err
	}
	//
	for _, function := range usedFuncMap {
		funcKey := GetFuncKey(function)
		node := &model.Node{
			NodeKey:   funcKey,
			NodeLabel: abridgeFuncName(funcKey),
			SsaFunc:   function,
			NodeType:  "func",
			EdgesOut:  make(map[string]*model.CallEdge),
			EdgesIn:   make(map[string]*model.CallEdge),
		}
		retCallGraph.NodeMap[node.NodeKey] = node
	}
	for callerFuncKey, node := range retCallGraph.NodeMap {
		ssaNode := callGraph.Nodes[node.SsaFunc]
		for _, edge := range ssaNode.Out {
			calleeFuncKey := GetFuncKey(edge.Callee.Func)
			calleeNode := retCallGraph.NodeMap[calleeFuncKey]
			if calleeNode != nil && callerFuncKey != calleeFuncKey {
				node.EdgesOut[calleeFuncKey] = &model.CallEdge{
					CalleeKey: calleeFuncKey,
					CallerKey: callerFuncKey,
					Dotted:    edge.Site == nil,
				}
				if edge.Site != nil {
					node.EdgesOut[calleeFuncKey].Args = edge.Site.Common().Args
				}
				if edge.Pos() != 0 {
					node.EdgesOut[calleeFuncKey].Pos = edge.Pos()
				}
				calleeNode.EdgesOut[callerFuncKey] = node.EdgesOut[calleeFuncKey]
			}
		}
	}
	return retCallGraph, nil
}

func abridgeFuncName(funcName string) string {
	if strings.Contains(funcName, "(") {
		begin := strings.Index(funcName, "(")
		end := strings.LastIndex(funcName, ")")
		return abridgeFuncName(funcName[0:begin]) + "(" + abridgeFuncName(funcName[begin+1:end]) + funcName[end:]
	}
	funcName = strings.ReplaceAll(funcName, "github.org", "g")
	items := strings.Split(funcName, "/")
	result := ""
	for i, item := range items {
		if i == len(items)-1 {
			result += item
		} else {
			if item[0] == '*' {
				result += item[0:2] + "/"
			} else {
				result += string(item[0]) + "/"
			}
		}
	}
	return result
}

func GetFuncKey(function *ssa.Function) string {
	return function.Pkg.Pkg.Path() + innerGetFuncKey(function)
}

func innerGetFuncKey(function *ssa.Function) string {
	funcKey := ""
	if function.Parent() != nil {
		funcKey += fmt.Sprintf("[%s]", innerGetFuncKey(function.Parent()))
	}
	if function.Signature.Recv() != nil {
		funcKey += fmt.Sprintf("(%s)", function.Signature.Recv().Type().String())
	}
	name := function.Name()
	if IsAnonymousFunc(function) {
		hash, _ := GetFuncHash(function)
		name = name[0:strings.LastIndex(name, "$")+1] + fmt.Sprintf("%x", md5.Sum(hash))
	}
	return fmt.Sprintf("%s#%s", funcKey, name)
}

func IsAnonymousFunc(function *ssa.Function) bool {
	return anonymousFuncNameRegex.MatchString(function.Name())
}

func GetFuncHash(function *ssa.Function) ([]byte, string) {
	var b bytes.Buffer
	ssa.WriteFunction(&b, function)
	lines := strings.Split(b.String(), "\n")
	resultString := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "func ") && IsAnonymousFunc(function) {
			line = anonymousFuncLIneNumRegex.ReplaceAllString(line, "$*(")
		}
		line = FieldAddrReg.ReplaceAllString(line, "[#?]")
		line = strings.ReplaceAll(line, " ", "")
		resultString += line + "\n"
	}
	hash := sha256.New()
	hash.Write([]byte(resultString))
	return hash.Sum(nil), resultString
}

func FilterUsedSsaGraphFunctions(graph *callgraph.Graph, rootPkg string) ([]*ssa.Function, error) {
	lib := GetStdLib(graph.Nodes)
	// 1.保留所有使用到的函数
	valuableFuncMap := make(map[*ssa.Function]struct{})
	for _, node := range graph.Nodes {
		if _, ok := valuableFuncMap[node.Func]; ok {
			continue
		}
		if node.Func.Pkg == nil {
			continue
		}
		if isCurrentRepoFunc(node.Func.Pkg.Pkg.Path(), rootPkg) {
			valuableFuncMap[node.Func] = struct{}{}
		}
	}
	// 2.保留所有依赖的函数
	dependsFuncMap := make(map[*ssa.Function]struct{})
	for _, node := range graph.Nodes {
		if !isUsefulFunc(node.Func, lib) {
			continue
		}
		for _, edge := range node.In {
			caller := edge.Caller
			if _, ok := valuableFuncMap[caller.Func]; ok {
				dependsFuncMap[node.Func] = struct{}{}
			}
		}
	}
	// 3.将依赖函数添加到有用函数集合中
	for function, _ := range dependsFuncMap {
		valuableFuncMap[function] = struct{}{}
	}
	valueFuncList := make([]*ssa.Function, 0)
	for function, _ := range valuableFuncMap {
		valueFuncList = append(valueFuncList, function)
	}
	return valueFuncList, nil
}

func GetStdLib(nodes map[*ssa.Function]*callgraph.Node) map[string]bool {
	allPackage := make(map[string]bool)
	for function := range nodes {
		if function.Pkg == nil {
			continue
		}
		allPackage[function.Pkg.Pkg.Path()] = true
	}
	return make(map[string]bool)
}

func VisitNode(node *callgraph.Node, visitFunc map[int]struct{}) {
	if node == nil {
		fmt.Println("new call graph")
		return
	}
	funcString := node.Func.String()
	visitFunc[node.ID] = struct{}{}
	if len(node.In) > 0 {
		for _, edge := range node.In {
			fmt.Printf("func: %s, in: %v\t", funcString, edge.String())
			VisitNode(edge.Callee, visitFunc)
			VisitNode(edge.Caller, visitFunc)
		}
	}
	if len(node.Out) > 0 {
		for _, edge := range node.Out {
			fmt.Printf("func: %s, out: %v\t", funcString, edge.String())
			VisitNode(edge.Callee, visitFunc)
			VisitNode(edge.Caller, visitFunc)
		}
	}
}
