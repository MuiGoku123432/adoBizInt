package ado

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// sdkModule is the Azure DevOps SDK root package; every sub-client package
// lives under it.
const sdkModule = "github.com/microsoft/azure-devops-go-api/azuredevops/v7"

// minSubClientConstructions is the NewClientWithOptions calls NewClient makes:
// the discovery root, workitemtracking, git, build, release, work and location.
const minSubClientConstructions = 7

// TestSubClientsUseInjectedClient pins D-11: every SDK sub-client in this
// package is built on the injected, org-bound HTTP client. It fails on any
// GetClientByUrl or GetClientByResourceAreaId selector, on any call to an SDK
// package's NewClient (azuredevops.NewClient and every typed constructor such
// as git.NewClient or location.NewClient), and on any
// azuredevops.NewClientWithOptions call without an azuredevops.WithHTTPClient
// option. Each of those builds a default http.Client that bypasses the
// transport, so the PAT would leave the process without the host rule.
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments and string literals are not part of
// the AST, so neither a doc line nor a message string can satisfy or defeat this
// assertion.
func TestSubClientsUseInjectedClient(t *testing.T) {
	fset, files := parsePackageFiles(t)
	constructions := 0
	for _, file := range files {
		findings, n := subClientFindings(fset, file)
		for _, f := range findings {
			t.Error(f)
		}
		constructions += n
	}

	if files["client.go"] == nil {
		t.Errorf("parsed files do not include client.go; the structural assertion covered nothing")
	}
	if constructions < minSubClientConstructions {
		t.Errorf("found %d azuredevops.NewClientWithOptions calls, want at least %d (root, workitemtracking, git, build, release, work, location); the assertion would be vacuous",
			constructions, minSubClientConstructions)
	}
}

// subClientFindings reports every D-11 violation in file and counts its
// azuredevops.NewClientWithOptions calls. Package selectors are resolved
// through the file's imports, so an alias cannot hide one. Each top-level
// declaration is one scope: an identifier counts as the injected option only
// when it is assigned from azuredevops.WithHTTPClient in the same function.
func subClientFindings(fset *token.FileSet, file *ast.File) (findings []string, constructions int) {
	pkgOf := map[string]string{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path != sdkModule && !strings.HasPrefix(path, sdkModule+"/") {
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if path == sdkModule {
			local = "azuredevops" // the v7 package is named azuredevops, not v7
		}
		if imp.Name != nil {
			local = imp.Name.Name
		}
		pkgOf[local] = path
	}

	// sdkCall reports the SDK package path and function name of a call to an
	// exported function of an SDK package.
	sdkCall := func(expr ast.Expr) (path, name string, ok bool) {
		call, isCall := expr.(*ast.CallExpr)
		if !isCall {
			return "", "", false
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			return "", "", false
		}
		x, isIdent := sel.X.(*ast.Ident)
		if !isIdent || pkgOf[x.Name] == "" {
			return "", "", false
		}
		return pkgOf[x.Name], sel.Sel.Name, true
	}
	isWithHTTPClient := func(expr ast.Expr) bool {
		path, name, ok := sdkCall(expr)
		return ok && path == sdkModule && name == "WithHTTPClient"
	}

	for _, decl := range file.Decls {
		injected := map[string]bool{}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i, rhs := range n.Rhs {
						if id, ok := n.Lhs[i].(*ast.Ident); ok && isWithHTTPClient(rhs) {
							injected[id.Name] = true
						}
					}
				}
			case *ast.ValueSpec:
				for i, v := range n.Values {
					if i < len(n.Names) && isWithHTTPClient(v) {
						injected[n.Names[i].Name] = true
					}
				}
			}
			return true
		})

		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if n.Sel.Name == "GetClientByUrl" || n.Sel.Name == "GetClientByResourceAreaId" {
					findings = append(findings, fset.Position(n.Pos()).String()+": "+n.Sel.Name+
						" builds a default http.Client that bypasses the transport")
				}
			case *ast.CallExpr:
				path, name, ok := sdkCall(n)
				if !ok {
					return true
				}
				switch {
				case name == "NewClient":
					findings = append(findings, fset.Position(n.Pos()).String()+": "+path+".NewClient"+
						" builds a default http.Client that bypasses the transport")
				case path == sdkModule && name == "NewClientWithOptions":
					constructions++
					hasOption := false
					for _, arg := range n.Args {
						if id, ok := arg.(*ast.Ident); ok && injected[id.Name] {
							hasOption = true
						}
						if isWithHTTPClient(arg) {
							hasOption = true
						}
					}
					if !hasOption {
						findings = append(findings, fset.Position(n.Pos()).String()+
							": NewClientWithOptions without an azuredevops.WithHTTPClient option builds a default http.Client")
					}
				}
			}
			return true
		})
	}
	return findings, constructions
}
