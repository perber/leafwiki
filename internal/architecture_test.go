package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/perber/wiki/"

// allowedSliceImports lists the internal/wiki slices other slices may import.
// pagesave is the shared page-mutation pipeline every mutating slice uses.
var allowedSliceImports = map[string]bool{
	"internal/wiki/pagesave": true,
}

// knownExceptions are existing violations with a cleanup plan. Don't add to
// this list; remove an entry once the import is gone.
var knownExceptions = map[string]bool{
	// apikeys reuses wiki/auth's error mapping until the shared error helper
	// exists (see docs/development/conventions.md#errors).
	"internal/wiki/apikeys -> internal/wiki/auth": true,
}

// TestArchitecture_ImportBoundaries_FollowLayerRules enforces the layer rules
// from docs/development/conventions.md#layers-and-where-code-goes.
func TestArchitecture_ImportBoundaries_FollowLayerRules(t *testing.T) {
	imports := collectProductionImports(t, ".")
	if len(imports) == 0 {
		t.Fatal("no packages found; test must run from the internal directory")
	}

	var violations []string
	for pkg, deps := range imports {
		for _, dep := range deps {
			if reason := violation(pkg, dep); reason != "" {
				edge := pkg + " -> " + dep
				if knownExceptions[edge] {
					continue
				}
				violations = append(violations, edge+": "+reason)
			}
		}
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Error(v)
	}
}

// scopeUnawarePackages must not know about scoped reads (ADR-0015): the scoped
// read layer is built on top of them, never the other way round. internal/core
// is covered by its own layer rule.
var scopeUnawarePackages = []string{
	"internal/links",
	"internal/search",
	"internal/tags",
	"internal/properties",
}

// violation returns why pkg must not import dep, or "" if the import is fine.
// Both are module-relative paths like "internal/core/tree"; dep may also be a
// third-party import path.
func violation(pkg, dep string) string {
	if isUnder(dep, "internal/scopedread") {
		for _, p := range scopeUnawarePackages {
			if isUnder(pkg, p) {
				return "indexes must not depend on the scoped read layer (ADR-0015)"
			}
		}
	}
	switch {
	case isUnder(pkg, "internal/core"):
		if strings.HasPrefix(dep, "github.com/gin-gonic/") {
			return "internal/core must not depend on gin"
		}
		if strings.HasPrefix(dep, "internal/") && !isUnder(dep, "internal/core") {
			return "internal/core may only import other internal/core packages"
		}
	case isUnder(pkg, "internal/http"):
		if isUnder(dep, "internal/wiki") {
			return "internal/http must not import internal/wiki"
		}
	case isUnder(pkg, "internal/wiki"):
		if pkg == "internal/wiki" || !isUnder(dep, "internal/wiki") || dep == "internal/wiki" {
			return ""
		}
		if sliceOf(pkg) != sliceOf(dep) && !allowedSliceImports[dep] {
			return "slices must not import other slices (wire them in internal/wiki/wiki.go)"
		}
	case strings.HasPrefix(pkg, "internal/"):
		if isUnder(dep, "internal/wiki") || isUnder(dep, "internal/http") {
			return "packages below internal/wiki must not import internal/wiki or internal/http"
		}
	}
	return ""
}

func isUnder(pkg, root string) bool {
	return pkg == root || strings.HasPrefix(pkg, root+"/")
}

// sliceOf returns "internal/wiki/<slice>" for any package inside a slice.
func sliceOf(pkg string) string {
	parts := strings.SplitN(pkg, "/", 4)
	if len(parts) < 3 {
		return pkg
	}
	return strings.Join(parts[:3], "/")
}

// collectProductionImports maps every package under root (module-relative,
// e.g. "internal/core/tree") to the imports of its non-test files. Module
// imports are returned module-relative, others unchanged.
func collectProductionImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	result := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "testdata" || name == "dist" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			return err
		}
		pkg := "internal"
		if dir := filepath.ToSlash(filepath.Dir(path)); dir != "." {
			pkg += "/" + dir
		}
		for _, spec := range file.Imports {
			dep, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			result[pkg] = append(result[pkg], strings.TrimPrefix(dep, modulePath))
		}
		if _, ok := result[pkg]; !ok {
			result[pkg] = nil
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return result
}

func TestArchitecture_Violation_DetectsForbiddenImports(t *testing.T) {
	cases := []struct {
		pkg, dep      string
		wantViolation bool
	}{
		{"internal/core/tree", "internal/core/markdown", false},
		{"internal/core/tree", "github.com/gin-gonic/gin", true},
		{"internal/core/tree", "internal/links", true},
		{"internal/core/tree", "internal/wiki/pages", true},
		{"internal/links", "internal/core/tree", false},
		{"internal/links", "internal/http/dto", true},
		{"internal/links", "internal/wiki", true},
		{"internal/http/middleware/auth", "internal/core/auth", false},
		{"internal/http", "internal/wiki/pages", true},
		{"internal/wiki/pages", "internal/wiki/pagesave", false},
		{"internal/wiki/pages", "internal/wiki/assets", true},
		{"internal/wiki/pages/sub", "internal/wiki/pages", false},
		{"internal/wiki", "internal/wiki/pages", false},
		{"internal/wiki/pages", "internal/wiki", false},
		{"internal/wiki/pages", "internal/core/tree", false},
		{"internal/core/tree", "internal/scopedread", true},
		{"internal/search", "internal/scopedread", true},
		{"internal/links", "internal/scopedread", true},
		{"internal/tags", "internal/scopedread", true},
		{"internal/properties", "internal/scopedread", true},
		{"internal/scopedread", "internal/core/tree", false},
		{"internal/wiki/pages", "internal/scopedread", false},
	}
	for _, tc := range cases {
		got := violation(tc.pkg, tc.dep) != ""
		if got != tc.wantViolation {
			t.Errorf("violation(%q, %q) = %v, want %v", tc.pkg, tc.dep, got, tc.wantViolation)
		}
	}
}
