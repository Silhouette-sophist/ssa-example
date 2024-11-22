package main

import (
	"fmt"
	"ssa-example/service"
)

func main() {
	graph, err := service.CallGraph("/Users/silhouette/work-practice/gin-example")
	if err != nil {
		fmt.Printf("createSsaCallGraph error %v\n", err)
		return
	}
	fmt.Printf("createSsaCallGraph callGraph: %v\n", graph)
}
