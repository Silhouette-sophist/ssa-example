package visitor

import "go/ast"

type FuncUsedScanner struct {
	IdentMap map[string]struct{}
}

func (fus *FuncUsedScanner) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Ident:
		if n.IsExported() {
			fus.IdentMap[n.Name] = struct{}{}
		}
	}
	return fus
}
