package cli

import (
	"testing"
)

func TestRootHasVerboseFlag(t *testing.T) {
	root := newRootCmd("test")
	f := root.PersistentFlags().Lookup("verbose")
	if f == nil {
		t.Fatal("root must expose a persistent --verbose flag")
	}
	if f.Shorthand != "v" {
		t.Fatalf("verbose shorthand must be -v, got %q", f.Shorthand)
	}
}
