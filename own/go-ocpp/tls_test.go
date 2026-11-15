// SPDX-License-Identifier: MIT
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-ocpp/server"
)

const testPassword = "0123456789abcdef"

func TestBasicAuth(t *testing.T) {
	for _, c := range []struct {
		name           string
		user, password string
		noBasic        bool
		want           bool
	}{
		{name: "the station's credentials", user: "CP001", password: testPassword, want: true},
		{name: "another password of the same length", user: "CP001", password: "0123456789abcdeX"},
		{name: "a prefix of the password", user: "CP001", password: testPassword[:8]},
		{name: "a longer password", user: "CP001", password: testPassword + "0"},
		{name: "an empty password", user: "CP001", password: ""},
		{name: "another user name", user: "CP002", password: testPassword},
		{name: "a prefix of the user name", user: "CP00", password: testPassword},
		{name: "no Basic credentials", noBasic: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := server.ConnectRequest{StationID: "CP001"}
			if !c.noBasic {
				r.Username, r.Password, r.HasBasicAuth = c.user, server.Secret(c.password), true
			}
			if got := basicAuthOK(r, "CP001", testPassword); got != c.want {
				t.Fatalf("basicAuthOK = %v, want %v", got, c.want)
			}
		})
	}
}

// TestCredentialsUseConstantTimeComparison: a Basic user name or password is
// compared through passwordOK only, never with == or !=.
func TestCredentialsUseConstantTimeComparison(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go files: %v", err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if b, ok := n.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) &&
				(credential(b.X) || credential(b.Y)) {
				t.Errorf("%s: a credential is compared with %s; use passwordOK", fset.Position(b.Pos()), b.Op)
			}
			return true
		})
	}
}

// credential reports whether an expression holds a Basic user name or password.
func credential(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			name := strings.ToLower(n.Name)
			found = found || name == "user" || name == "username" || strings.Contains(name, "password")
		case *ast.SelectorExpr:
			name := strings.ToLower(n.Sel.Name)
			found = found || name == "username" || strings.Contains(name, "password")
		}
		return !found
	})
	return found
}
