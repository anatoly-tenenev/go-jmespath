package jmespath

import (
	"sort"
	"strconv"
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
	case ASTField, ASTSubexpression, ASTIndexExpression:
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
		return guardPathSetFromComparatorWhenTrue(node)
	case ASTFunctionExpression:
		return guardPathSetFromFunctionWhenTrue(node)
	default:
		return nil
	}
}

func guardPathSetFromFunctionWhenTrue(node ASTNode) guardPathSet {
	if node.nodeType != ASTFunctionExpression {
		return nil
	}
	name, ok := node.value.(string)
	if !ok {
		return nil
	}
	if len(node.children) != 2 {
		return nil
	}

	switch name {
	case "starts_with", "ends_with":
		return unionGuardPathSets(
			guardPathSetFromPathNode(node.children[0]),
			guardPathSetFromPathNode(node.children[1]),
		)
	case "contains":
		leftGuards := guardPathSetFromPathNode(node.children[0])
		if !containsGuardsSecondArgWhenTrue(node.children[0]) {
			return leftGuards
		}
		rightGuards := guardPathSetFromPathNode(node.children[1])
		return unionGuardPathSets(leftGuards, rightGuards)
	default:
		return nil
	}
}

func containsGuardsSecondArgWhenTrue(searchArg ASTNode) bool {
	if isStringLiteralNode(searchArg) {
		return true
	}
	return isLiteralArrayWithoutNull(searchArg)
}

func isStringLiteralNode(node ASTNode) bool {
	_, ok := node.value.(string)
	return node.nodeType == ASTLiteral && ok
}

func isLiteralArrayWithoutNull(node ASTNode) bool {
	switch node.nodeType {
	case ASTLiteral:
		items, ok := node.value.([]interface{})
		if !ok {
			return false
		}
		for _, item := range items {
			if item == nil {
				return false
			}
		}
		return true
	case ASTMultiSelectList:
		for _, child := range node.children {
			if child.nodeType != ASTLiteral || child.value == nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func guardPathSetFromComparatorWhenTrue(node ASTNode) guardPathSet {
	if len(node.children) != 2 {
		return nil
	}
	operator, ok := node.value.(tokType)
	if !ok {
		return nil
	}
	left := node.children[0]
	right := node.children[1]

	switch operator {
	case tNE:
		if isNullLiteralNode(left) {
			return guardPathSetFromPathNode(right)
		}
		if isNullLiteralNode(right) {
			return guardPathSetFromPathNode(left)
		}
		return nil
	case tEQ:
		if isNonNullLiteralNode(left) {
			return guardPathSetFromPathNode(right)
		}
		if isNonNullLiteralNode(right) {
			return guardPathSetFromPathNode(left)
		}
		return nil
	case tLT, tLTE, tGT, tGTE:
		return unionGuardPathSets(
			guardPathSetFromPathNode(left),
			guardPathSetFromPathNode(right),
		)
	default:
		return nil
	}
}

func guardPathSetFromPathNode(node ASTNode) guardPathSet {
	path, ok := narrowableSchemaPath(node)
	if !ok || path == "" || path == "@" {
		return nil
	}
	paths := guardPathPrefixes(path)
	if len(paths) == 0 {
		return nil
	}
	return paths
}

func narrowableSchemaPath(node ASTNode) (string, bool) {
	switch node.nodeType {
	case ASTField:
		name, ok := node.value.(string)
		if !ok || !isNarrowableFieldName(name) {
			return "", false
		}
		return name, true
	case ASTSubexpression:
		if len(node.children) != 2 {
			return "", false
		}
		leftPath, ok := narrowableSchemaPath(node.children[0])
		if !ok {
			return "", false
		}
		rightPath, ok := narrowableSchemaPath(node.children[1])
		if !ok {
			return "", false
		}
		if leftPath == "@" {
			return rightPath, true
		}
		if rightPath == "@" {
			return leftPath, true
		}
		return leftPath + "." + rightPath, true
	case ASTIndexExpression:
		if len(node.children) != 2 {
			return "", false
		}
		basePath, ok := narrowableSchemaPath(node.children[0])
		if !ok {
			return "", false
		}
		indexNode := node.children[1]
		switch indexNode.nodeType {
		case ASTIndex:
			index, ok := indexNode.value.(int)
			if !ok {
				return "", false
			}
			return basePath + "[" + strconv.Itoa(index) + "]", true
		case ASTSlice:
			return basePath + "[]", true
		default:
			return "", false
		}
	case ASTIdentity, ASTCurrentNode:
		return "@", true
	default:
		return "", false
	}
}

func isNarrowableFieldName(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsAny(name, ".[]")
}

func guardPathPrefixes(path string) guardPathSet {
	if path == "" || path == "@" {
		return nil
	}
	paths := make(guardPathSet)
	prefix := ""
	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch ch {
		case '.':
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
		case '[':
			if prefix != "" {
				paths[prefix] = struct{}{}
			}
		}
		prefix += string(ch)
	}
	if prefix != "" {
		paths[prefix] = struct{}{}
	}
	return paths
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

func isNonNullLiteralNode(node ASTNode) bool {
	return node.nodeType == ASTLiteral && node.value != nil
}
