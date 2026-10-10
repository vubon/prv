package diagram

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// Node represents a package component in the architecture diagram.
type Node struct {
	ID        string `json:"id"`
	Package   string `json:"package"`
	IsTouched bool   `json:"is_touched"`
}

// Edge represents a directed dependency link between packages.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph holds packages and their directional relationships.
type Graph struct {
	Nodes map[string]Node
	Edges map[string]Edge // Key: "From->To"
}

// NewGraph creates an empty dependency graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]Node),
		Edges: make(map[string]Edge),
	}
}

// AddNode registers a package node in the graph.
func (g *Graph) AddNode(pkg string, isTouched bool) {
	pkg = normalizePackage(pkg)
	if pkg == "" || pkg == "." {
		return
	}
	id := SanitizeID(pkg)
	if existing, exists := g.Nodes[pkg]; exists {
		if isTouched {
			existing.IsTouched = true
			g.Nodes[pkg] = existing
		}
		return
	}
	g.Nodes[pkg] = Node{
		ID:        id,
		Package:   pkg,
		IsTouched: isTouched,
	}
}

// AddEdge registers a dependency relationship from -> to.
func (g *Graph) AddEdge(from, to string) {
	from = normalizePackage(from)
	to = normalizePackage(to)
	if from == "" || to == "" || from == to {
		return
	}
	g.AddNode(from, false)
	g.AddNode(to, false)
	key := fmt.Sprintf("%s->%s", from, to)
	g.Edges[key] = Edge{From: from, To: to}
}

// ToMermaid generates a clean, valid Mermaid LR graph string.
func (g *Graph) ToMermaid() string {
	if len(g.Edges) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("graph LR\n")

	// Sort node keys for deterministic rendering
	nodeKeys := make([]string, 0, len(g.Nodes))
	for k := range g.Nodes {
		nodeKeys = append(nodeKeys, k)
	}
	sort.Strings(nodeKeys)

	// Render node labels
	for _, k := range nodeKeys {
		node := g.Nodes[k]
		fmt.Fprintf(&sb, "    %s[\"%s\"]\n", node.ID, node.Package)
	}

	sb.WriteString("\n")

	// Sort edge keys for deterministic rendering
	edgeKeys := make([]string, 0, len(g.Edges))
	for k := range g.Edges {
		edgeKeys = append(edgeKeys, k)
	}
	sort.Strings(edgeKeys)

	// Render edges
	for _, k := range edgeKeys {
		edge := g.Edges[k]
		fromNode := g.Nodes[edge.From]
		toNode := g.Nodes[edge.To]
		fmt.Fprintf(&sb, "    %s --> %s\n", fromNode.ID, toNode.ID)
	}

	// Highlight modified packages with distinct styling
	var touchedIDs []string
	for _, k := range nodeKeys {
		node := g.Nodes[k]
		if node.IsTouched {
			touchedIDs = append(touchedIDs, node.ID)
		}
	}
	if len(touchedIDs) > 0 {
		sb.WriteString("\n")
		sb.WriteString("    classDef touched fill:#e1f5fe,stroke:#0288d1,stroke-width:2px;\n")
		fmt.Fprintf(&sb, "    class %s touched;\n", strings.Join(touchedIDs, ","))
	}

	return sb.String()
}

// SanitizeID converts package paths into valid Mermaid node identifiers.
func SanitizeID(pkg string) string {
	cleaned := strings.ReplaceAll(pkg, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	cleaned = strings.ReplaceAll(cleaned, ".", "_")
	cleaned = strings.ReplaceAll(cleaned, "@", "_")
	if len(cleaned) > 0 && cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "pkg_" + cleaned
	}
	if cleaned == "" {
		cleaned = "root"
	}
	return cleaned
}

func normalizePackage(pkg string) string {
	pkg = filepath.ToSlash(filepath.Clean(pkg))
	pkg = strings.TrimPrefix(pkg, "./")
	pkg = strings.Trim(pkg, "/")
	return pkg
}

// FindModulePath inspects repoPath for a go.mod file and returns the declared module name.
func FindModulePath(repoPath string) string {
	if repoPath == "" {
		repoPath = "."
	}
	modFile := filepath.Join(repoPath, "go.mod")
	data, err := os.ReadFile(modFile)
	if err != nil {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

// GenerateMermaid analyzes changed files and their package relationships to produce a Mermaid diagram.
func GenerateMermaid(repoPath string, files []models.FileChange) string {
	if len(files) == 0 {
		return ""
	}

	graph := BuildGraph(repoPath, files)
	if graph == nil {
		return ""
	}

	return graph.ToMermaid()
}

// BuildGraph builds a dependency graph from changed files and their package boundaries.
func BuildGraph(repoPath string, files []models.FileChange) *Graph {
	g := NewGraph()
	if repoPath == "" {
		repoPath = "."
	}

	modulePath := FindModulePath(repoPath)

	// Collect touched packages
	touchedPkgs := make(map[string]bool)
	for _, f := range files {
		if f.IsFiltered {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(f.Path))
		if dir == "." || dir == "" {
			continue
		}
		touchedPkgs[dir] = true
		g.AddNode(dir, true)
	}

	if len(touchedPkgs) == 0 {
		return g
	}

	// 1. Parse Go AST imports for touched packages
	parseGoImports(repoPath, files, touchedPkgs, modulePath, g)

	// 2. Fallback / Polyglot import parsing (JS/TS/Python) if no edges detected yet
	if len(g.Edges) == 0 {
		parsePolyglotImports(repoPath, files, touchedPkgs, g)
	}

	return g
}

func parseGoImports(repoPath string, files []models.FileChange, touchedPkgs map[string]bool, modulePath string, g *Graph) {
	fset := token.NewFileSet()

	// Parse imports for all modified Go files
	for _, f := range files {
		if f.IsFiltered || !strings.HasSuffix(f.Path, ".go") {
			continue
		}

		srcPkg := filepath.ToSlash(filepath.Dir(f.Path))
		fullPath := filepath.Join(repoPath, f.Path)

		// Parse AST on disk if file exists
		if _, err := os.Stat(fullPath); err == nil {
			node, parseErr := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
			if parseErr == nil && node != nil {
				for _, imp := range node.Imports {
					importPath := strings.Trim(imp.Path.Value, `"`)
					processGoImport(importPath, modulePath, srcPkg, touchedPkgs, g)
				}
				continue
			}
		}

		// Fallback: parse import lines from diff hunk if file not on disk (e.g. remote PR)
		if f.DiffHunk != "" {
			parseImportsFromDiff(f.DiffHunk, modulePath, srcPkg, touchedPkgs, g)
		}
	}

	// Also inspect other Go files in the touched package directories to detect package-level relationships
	for pkg := range touchedPkgs {
		pkgDir := filepath.Join(repoPath, pkg)
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			filePath := filepath.Join(pkgDir, entry.Name())
			node, parseErr := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
			if parseErr != nil || node == nil {
				continue
			}
			for _, imp := range node.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				processGoImport(importPath, modulePath, pkg, touchedPkgs, g)
			}
		}
	}
}

func processGoImport(importPath, modulePath, srcPkg string, touchedPkgs map[string]bool, g *Graph) {
	if modulePath != "" && strings.HasPrefix(importPath, modulePath+"/") {
		targetPkg := strings.TrimPrefix(importPath, modulePath+"/")
		targetPkg = normalizePackage(targetPkg)
		if targetPkg != srcPkg {
			isTargetTouched := touchedPkgs[targetPkg]
			// Include target if it's touched or if source is touched and target is internal
			g.AddNode(targetPkg, isTargetTouched)
			g.AddEdge(srcPkg, targetPkg)
		}
	}
}

var diffImportRegex = regexp.MustCompile(`\+?\s*"(.*?)"`)

func parseImportsFromDiff(diffHunk, modulePath, srcPkg string, touchedPkgs map[string]bool, g *Graph) {
	scanner := bufio.NewScanner(strings.NewReader(diffHunk))
	inImport := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "+import (") || strings.HasPrefix(line, "import (") {
			inImport = true
			continue
		}
		if inImport && strings.HasPrefix(line, ")") {
			inImport = false
			continue
		}
		if inImport || strings.HasPrefix(line, "+import \"") {
			matches := diffImportRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				processGoImport(matches[1], modulePath, srcPkg, touchedPkgs, g)
			}
		}
	}
}

var (
	jsImportRegex = regexp.MustCompile(`(?:import|from|require\()\s*['"]([./@][^'"]+)['"]`)
	pyImportRegex = regexp.MustCompile(`(?:from|import)\s+([a-zA-Z0-9_.]+)`)
)

func parsePolyglotImports(repoPath string, files []models.FileChange, touchedPkgs map[string]bool, g *Graph) {
	for _, f := range files {
		if f.IsFiltered {
			continue
		}
		srcPkg := filepath.ToSlash(filepath.Dir(f.Path))
		ext := strings.ToLower(filepath.Ext(f.Path))

		switch ext {
		case ".ts", ".tsx", ".js", ".jsx":
			scanner := bufio.NewScanner(strings.NewReader(f.DiffHunk))
			for scanner.Scan() {
				line := scanner.Text()
				matches := jsImportRegex.FindStringSubmatch(line)
				if len(matches) > 1 {
					targetRaw := matches[1]
					if strings.HasPrefix(targetRaw, ".") {
						resolved := filepath.ToSlash(filepath.Clean(filepath.Join(srcPkg, targetRaw)))
						resolvedDir := filepath.Dir(resolved)
						if resolvedDir != "." && resolvedDir != srcPkg {
							g.AddNode(resolvedDir, touchedPkgs[resolvedDir])
							g.AddEdge(srcPkg, resolvedDir)
						}
					}
				}
			}
		case ".py":
			scanner := bufio.NewScanner(strings.NewReader(f.DiffHunk))
			for scanner.Scan() {
				line := scanner.Text()
				matches := pyImportRegex.FindStringSubmatch(line)
				if len(matches) > 1 {
					pyTarget := strings.ReplaceAll(matches[1], ".", "/")
					if touchedPkgs[pyTarget] && pyTarget != srcPkg {
						g.AddNode(pyTarget, true)
						g.AddEdge(srcPkg, pyTarget)
					}
				}
			}
		}
	}
}
