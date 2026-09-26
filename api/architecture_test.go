package main

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "supportchat/"

// allowedInternalImports is the dependency rule from docs/adr/0001-clean-architecture-dependency-rule.md.
// Packages listed here may import only the internal packages in their list, plus the standard library.
var allowedInternalImports = map[string][]string{
	module + "business/models":   {},
	module + "business/usecases": {module + "business/models"},
}

func TestDependencyRule(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg, imports := fields[0], fields[1:]
		allowed, ruled := allowedInternalImports[pkg]
		if !ruled {
			continue
		}
		for _, imp := range imports {
			internal := strings.HasPrefix(imp, module)
			stdlib := !strings.Contains(strings.Split(imp, "/")[0], ".")
			if (internal && !slices.Contains(allowed, imp)) || (!internal && !stdlib) {
				t.Errorf("%s must not import %s (ADR 0001)", pkg, imp)
			}
		}
	}
}
