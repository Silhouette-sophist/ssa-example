package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestCallGraph(t *testing.T) {
	rootDir := "/Users/silhouette/work-practice/gin-example"
	rootPkg := "gin-example"
	graph, err := CallGraph(rootDir)
	if err != nil {
		fmt.Printf("createSsaCallGraph CallGraph error %v\n", err)
		return
	}
	customGraph, err := ConvertSsaCallGraphToCustomGraph(graph, rootPkg)
	if err != nil {
		fmt.Printf("ConvertSsaCallGraphToCustomGraph CallGraph error %v\n", err)
		return
	}
	fmt.Printf("customGraph: %v", customGraph.NodeMap)
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
				VisitNode(node, visitFunc)
			}
		}
	}()
	// 等待goroutine完成或超时
	<-ctx.Done()
	fmt.Printf("visitFunc %v", visitFunc)
}

func TestCallGraphFacade(t *testing.T) {
	rootDir := "/Users/silhouette/work-practice/gin-example"
	rootPkg := "gin-example"
	facade, err := CallGraphFacade(rootDir, rootPkg)
	if err != nil {
		t.Errorf("CallGraphFacade error %v", err)
		return
	}
	marshal, err := json.Marshal(facade)
	if err != nil {
		t.Errorf("CallGraphFacade marshal error %v", err)
		return
	}
	t.Logf("CallGraphFacade result %v", string(marshal))
}
