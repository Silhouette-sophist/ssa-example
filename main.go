package main

import (
	"context"
	"fmt"
	"golang.org/x/tools/go/callgraph"
	"ssa-example/service"
	"time"
)

func main() {
	graph, err := service.CallGraph("/Users/silhouette/work-practice/gin-example")
	if err != nil {
		fmt.Printf("createSsaCallGraph CallGraph error %v\n", err)
		return
	}
	fmt.Println()
	customGraph, err := service.ConvertSsaCallGraphToCustomGraph(graph, "gin-example")
	if err != nil {
		fmt.Printf("ConvertSsaCallGraphToCustomGraph CallGraph error %v\n", err)
		return
	}
	fmt.Printf("customGraph: %v", customGraph)
	visitFunc := make(map[int]struct{})
	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		for _, node := range graph.Nodes {
			select {
			case <-ctx.Done():
				// 超时处理
				fmt.Println("Operation timed out")
				return
			default:
				visitNode(node, visitFunc)
			}
		}
	}()
	// 等待goroutine完成或超时
	<-ctx.Done()
	fmt.Printf("visitFunc %v", visitFunc)
}

func visitNode(node *callgraph.Node, visitFunc map[int]struct{}) {
	if node == nil {
		fmt.Println("new call graph")
		return
	}
	funcString := node.Func.String()
	visitFunc[node.ID] = struct{}{}
	if len(node.In) > 0 {
		for _, edge := range node.In {
			fmt.Printf("func: %s, in: %v\t", funcString, edge.String())
			visitNode(edge.Callee, visitFunc)
			visitNode(edge.Caller, visitFunc)
		}
	}
	if len(node.Out) > 0 {
		for _, edge := range node.Out {
			fmt.Printf("func: %s, out: %v\t", funcString, edge.String())
			visitNode(edge.Callee, visitFunc)
			visitNode(edge.Caller, visitFunc)
		}
	}
}
