package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ad3n/jsonschema"
)

type (
	DependencyGraph struct {
		nodes  map[string]*StructNode
		edges  map[string][]string
		cycles [][]string
	}

	StructNode struct {
		IsReferenced bool
	}

	ReferenceAnalyzer struct {
		graph *DependencyGraph
	}
)

func NewReferenceAnalyzer() *ReferenceAnalyzer {
	return &ReferenceAnalyzer{
		graph: &DependencyGraph{
			nodes: make(map[string]*StructNode),
			edges: make(map[string][]string),
		},
	}
}

func (ra *ReferenceAnalyzer) AnalyzePackageDependencies(structInfos []*GenerationInfo) error {
	for _, structInfo := range structInfos {
		ra.graph.nodes[structInfo.Name] = &StructNode{}
	}

	for _, structInfo := range structInfos {
		if err := ra.analyzeStructDependencies(structInfo); err != nil {
			return fmt.Errorf("failed to analyze dependencies for struct %s: %w", structInfo.Name, err)
		}
	}

	return ra.detectCircularDependencies()
}

func (ra *ReferenceAnalyzer) analyzeStructDependencies(structInfo *GenerationInfo) error {
	if ra.graph.nodes[structInfo.Name] == nil {
		return fmt.Errorf("%w: %s", jsonschema.ErrStructNodeNotFound, structInfo.Name)
	}

	for i := range structInfo.Fields {
		dependency := ra.analyzeTypeString(structInfo.Fields[i].TypeName)
		if dependency == "" {
			continue
		}

		ra.addEdge(structInfo.Name, dependency)
		if node := ra.graph.nodes[dependency]; node != nil {
			node.IsReferenced = true
		}
	}

	return nil
}

func (ra *ReferenceAnalyzer) analyzeTypeString(typeName string) string {
	typeName = strings.TrimPrefix(typeName, "*")
	if element, ok := strings.CutPrefix(typeName, "[]"); ok {
		element = strings.TrimPrefix(element, "*")
		if ra.isCustomStruct(element) {
			return element
		}
	}

	if mapType, ok := strings.CutPrefix(typeName, "map["); ok {
		if end := strings.LastIndex(mapType, "]"); end != -1 && end < len(mapType)-1 {
			value := strings.TrimPrefix(mapType[end+1:], "*")
			if ra.isCustomStruct(value) {
				return value
			}
		}
	}

	if ra.isCustomStruct(typeName) {
		return typeName
	}

	return ""
}

func (ra *ReferenceAnalyzer) isCustomStruct(typeName string) bool {
	builtinTypes := map[string]bool{
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true,
	}

	if builtinTypes[typeName] {
		return false
	}

	if strings.Contains(typeName, "time.Time") ||
		strings.Contains(typeName, "json.") ||
		strings.Contains(typeName, "net.") ||
		strings.Contains(typeName, "http.") {
		return false
	}

	if len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
		return true
	}

	return false
}

func (ra *ReferenceAnalyzer) addEdge(from, to string) {
	if ra.graph.edges[from] == nil {
		ra.graph.edges[from] = []string{}
	}

	if slices.Contains(ra.graph.edges[from], to) {
		return
	}

	ra.graph.edges[from] = append(ra.graph.edges[from], to)
}

func (ra *ReferenceAnalyzer) detectCircularDependencies() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeName := range ra.graph.nodes {
		if !visited[nodeName] {
			cycle := ra.dfsDetectCycle(nodeName, visited, recStack, []string{})
			if cycle != nil {
				ra.graph.cycles = append(ra.graph.cycles, cycle)
			}
		}
	}

	return nil
}

func (ra *ReferenceAnalyzer) dfsDetectCycle(node string, visited, recStack map[string]bool, path []string) []string {
	visited[node] = true
	recStack[node] = true
	path = append(path, node)

	for _, neighbor := range ra.graph.edges[node] {
		if !visited[neighbor] {
			cycle := ra.dfsDetectCycle(neighbor, visited, recStack, path)
			if cycle != nil {
				return cycle
			}
			continue
		}

		if recStack[neighbor] {
			cycleStart := -1
			for i, p := range path {
				if p == neighbor {
					cycleStart = i
					break
				}
			}

			if cycleStart >= 0 {
				return append(path[cycleStart:], neighbor)
			}
		}
	}

	recStack[node] = false
	return nil
}

func (ra *ReferenceAnalyzer) HasCycles() bool {
	return len(ra.graph.cycles) > 0
}

func (ra *ReferenceAnalyzer) Cycles() [][]string {
	return ra.graph.cycles
}

func (ra *ReferenceAnalyzer) NeedsRefGeneration(structName string) bool {
	node := ra.graph.nodes[structName]
	if node == nil {
		return false
	}

	if node.IsReferenced {
		for _, cycle := range ra.graph.cycles {
			if slices.Contains(cycle, structName) {
				return true
			}
		}
	}

	return false
}

func (ra *ReferenceAnalyzer) ReferencedStructs(rootStruct string) []string {
	var referenced []string
	visited := make(map[string]bool)

	ra.collectReferencedStructs(rootStruct, visited, &referenced)

	return referenced
}

func (ra *ReferenceAnalyzer) collectReferencedStructs(structName string, visited map[string]bool, result *[]string) {
	if visited[structName] {
		return
	}

	visited[structName] = true

	for _, dep := range ra.graph.edges[structName] {
		if !visited[dep] {
			*result = append(*result, dep)
			ra.collectReferencedStructs(dep, visited, result)
		}
	}
}
