package jmespath

import (
	"sort"
	"strings"
)

// GuardAnalysis contains schema-aware guard guarantees for an expression.
// Instances are immutable after compile.
type GuardAnalysis struct {
	protectedPaths []string
	protectedSet   map[string]struct{}
}

// Protects reports whether path is guaranteed to be non-missing and non-null
// whenever the associated expression evaluates to true.
func (g *GuardAnalysis) Protects(path string) bool {
	if g == nil || path == "" {
		return false
	}
	_, ok := g.protectedSet[path]
	return ok
}

// ProtectedPaths returns all guarded paths in stable order.
// The returned slice is a copy.
func (g *GuardAnalysis) ProtectedPaths() []string {
	if g == nil || len(g.protectedPaths) == 0 {
		return nil
	}
	paths := make([]string, len(g.protectedPaths))
	copy(paths, g.protectedPaths)
	return paths
}

type guardPathSet map[string]struct{}

type guardAnalyzer struct{}

func analyzeGuardsWhenTrue(ast ASTNode) *GuardAnalysis {
	analyzer := guardAnalyzer{}
	return newGuardAnalysis(analyzer.analyzeWhenTrue(ast))
}

func newGuardAnalysis(paths guardPathSet) *GuardAnalysis {
	if len(paths) == 0 {
		return &GuardAnalysis{}
	}
	result := &GuardAnalysis{
		protectedPaths: make([]string, 0, len(paths)),
		protectedSet:   make(map[string]struct{}, len(paths)),
	}
	for path := range paths {
		result.protectedSet[path] = struct{}{}
		result.protectedPaths = append(result.protectedPaths, path)
	}
	sort.Strings(result.protectedPaths)
	return result
}

func (a *guardAnalyzer) analyzeWhenTrue(node ASTNode) guardPathSet {
	switch node.nodeType {
	case ASTField, ASTSubexpression:
		return guardPathSetFromPathNode(node)
	case ASTAndExpression:
		if len(node.children) != 2 {
			return nil
		}
		left := a.analyzeWhenTrue(node.children[0])
		right := a.analyzeWhenTrue(node.children[1])
		return unionGuardPathSets(left, right)
	case ASTOrExpression:
		if len(node.children) != 2 {
			return nil
		}
		left := a.analyzeWhenTrue(node.children[0])
		right := a.analyzeWhenTrue(node.children[1])
		return intersectGuardPathSets(left, right)
	case ASTNotExpression:
		return nil
	case ASTComparator:
		return guardPathSetFromNullComparator(node)
	default:
		return nil
	}
}

func guardPathSetFromNullComparator(node ASTNode) guardPathSet {
	if len(node.children) != 2 {
		return nil
	}
	operator, ok := node.value.(tokType)
	if !ok || operator != tNE {
		return nil
	}
	left := node.children[0]
	right := node.children[1]
	if isNullLiteralNode(left) {
		return guardPathSetFromPathNode(right)
	}
	if isNullLiteralNode(right) {
		return guardPathSetFromPathNode(left)
	}
	return nil
}

func guardPathSetFromPathNode(node ASTNode) guardPathSet {
	segments, ok := guardPathSegments(node)
	if !ok || len(segments) == 0 {
		return nil
	}
	paths := make(guardPathSet, len(segments))
	addGuardPathAndPrefixes(paths, segments)
	return paths
}

func guardPathSegments(node ASTNode) ([]string, bool) {
	switch node.nodeType {
	case ASTField:
		name, ok := node.value.(string)
		if !ok || !isGuardPathSegment(name) {
			return nil, false
		}
		return []string{name}, true
	case ASTSubexpression:
		if len(node.children) != 2 {
			return nil, false
		}
		left, ok := guardPathSegments(node.children[0])
		if !ok {
			return nil, false
		}
		right, ok := guardPathSegments(node.children[1])
		if !ok {
			return nil, false
		}
		combined := make([]string, 0, len(left)+len(right))
		combined = append(combined, left...)
		combined = append(combined, right...)
		return combined, true
	case ASTIdentity, ASTCurrentNode:
		return []string{}, true
	default:
		return nil, false
	}
}

func isGuardPathSegment(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsAny(name, ".[]")
}

func addGuardPathAndPrefixes(paths guardPathSet, segments []string) {
	prefix := ""
	for i, segment := range segments {
		if i == 0 {
			prefix = segment
		} else {
			prefix += "." + segment
		}
		paths[prefix] = struct{}{}
	}
}

func unionGuardPathSets(left, right guardPathSet) guardPathSet {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	merged := make(guardPathSet, len(left)+len(right))
	for path := range left {
		merged[path] = struct{}{}
	}
	for path := range right {
		merged[path] = struct{}{}
	}
	return merged
}

func intersectGuardPathSets(left, right guardPathSet) guardPathSet {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	var smaller, larger guardPathSet
	if len(left) <= len(right) {
		smaller, larger = left, right
	} else {
		smaller, larger = right, left
	}
	var intersection guardPathSet
	for path := range smaller {
		if _, ok := larger[path]; ok {
			if intersection == nil {
				intersection = make(guardPathSet, len(smaller))
			}
			intersection[path] = struct{}{}
		}
	}
	return intersection
}

func isNullLiteralNode(node ASTNode) bool {
	return node.nodeType == ASTLiteral && node.value == nil
}
