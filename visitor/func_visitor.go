package visitor

import "go/ast"

type FuncUsedScanner struct {
	IdentMap map[string]struct{}
}

// Visit ast遍历，只获取被导出的标识符
func (fus *FuncUsedScanner) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	// TODO 只识别导出的标识符[函数、变量、类型]
	case *ast.Ident:
		if n.IsExported() {
			fus.IdentMap[n.Name] = struct{}{}
		}
	}
	return fus
}
