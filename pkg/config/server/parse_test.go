package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileExpandsEnvironment(t *testing.T) {
	t.Setenv("TEST_INFRAOPS_MYSQL_DSN", "user:pass@tcp(mysql:3306)/infraops")
	path := filepath.Join(t.TempDir(), "server.yaml")
	contents := []byte("mysql:\n  - name: stree\n    addr: ${TEST_INFRAOPS_MYSQL_DSN}\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Mysql[0].Addr; got != "user:pass@tcp(mysql:3306)/infraops" {
		t.Fatalf("expanded DSN = %q", got)
	}
}
