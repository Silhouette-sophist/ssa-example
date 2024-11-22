package main

import (
	"fmt"
	"ssa-example/service"
)

func main() {
	graph, err := service.CallGraph("/Users/silhouette/work-practice/gin-example")
	if err != nil {
		fmt.Printf("createSsaCallGraph CallGraph error %v\n", err)
		return
	}
	fmt.Println()
	for function, node := range graph.Nodes {
		fmt.Printf("function name %s, in %v, out %v\n", function.Name(), node.In, node.Out)
	}
}
