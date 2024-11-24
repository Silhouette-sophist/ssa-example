package model

import (
	"go/token"
	"golang.org/x/tools/go/ssa"
)

type CallGraph struct {
	NodeMap map[string]*Node
	RootPkg string
}

type Node struct {
	NodeKey   string
	NodeLabel string
	SsaFunc   *ssa.Function
	EdgesOut  map[string]*CallEdge
	EdgesIn   map[string]*CallEdge
	DiffType  string
	NodeType  string
	Hash      string
}

type CallEdge struct {
	DiffType  string
	CallerKey string
	CalleeKey string
	Dotted    bool
	Args      []ssa.Value
	Pos       token.Pos
}
