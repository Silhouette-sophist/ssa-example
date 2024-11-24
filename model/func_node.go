package model

// SsaFunc 从ssa.Function中解析需要的数据
type SsaFunc struct {
	Name      string
	Params    []*SsaParam
	Signature string
	Blocks    []*Block
	CallEdges []*CallEdge
}

type SsaParam struct {
	Name   string
	Type   string
	Parent string
}

type Block struct {
}

type CallType int

const (
	CallIn = iota
	CallOut
)
