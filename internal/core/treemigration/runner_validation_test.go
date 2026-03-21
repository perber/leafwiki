package treemigration

import (
	"strings"
	"testing"
)

type testNode struct{}

func (testNode) ID() string           { return "root" }
func (testNode) Title() string        { return "root" }
func (testNode) Slug() string         { return "root" }
func (testNode) Kind() string         { return NodeKindSection }
func (testNode) SetKind(string)       {}
func (testNode) Metadata() Metadata   { return Metadata{} }
func (testNode) SetMetadata(Metadata) {}
func (testNode) Children() []Node     { return nil }

type testStore struct{}

func (testStore) ResolveNode(Node) (*ResolvedNode, error) {
	return &ResolvedNode{Kind: NodeKindSection}, nil
}
func (testStore) ContentPathForRead(Node) (string, error)  { return "", nil }
func (testStore) ContentPathForWrite(Node) (string, error) { return "", nil }
func (testStore) EnsureSectionIndex(Node) (string, error)  { return "", nil }
func (testStore) ReadPageRaw(Node) (string, error)         { return "", nil }

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Error(string, ...any) {}

func validDependencies() Dependencies {
	return Dependencies{
		Root:                 testNode{},
		Store:                testStore{},
		Log:                  testLogger{},
		CurrentSchemaVersion: 4,
		SaveTree:             func() error { return nil },
		SaveSchema:           func(int) error { return nil },
	}
}

func TestRun_RejectsNegativeFromVersion(t *testing.T) {
	deps := validDependencies()

	err := Run(-1, deps)
	if err == nil {
		t.Fatalf("expected error for negative schema version")
	}
	if !strings.Contains(err.Error(), "invalid schema version") {
		t.Fatalf("expected invalid schema version error, got: %v", err)
	}
}

func TestRun_RejectsMissingRequiredDependencies(t *testing.T) {
	tests := []struct {
		name string
		deps Dependencies
		want string
	}{
		{name: "nil root", deps: func() Dependencies { d := validDependencies(); d.Root = nil; return d }(), want: "tree not loaded"},
		{name: "nil store", deps: func() Dependencies { d := validDependencies(); d.Store = nil; return d }(), want: "migration store is required"},
		{name: "nil log", deps: func() Dependencies { d := validDependencies(); d.Log = nil; return d }(), want: "migration logger is required"},
		{name: "nil save tree", deps: func() Dependencies { d := validDependencies(); d.SaveTree = nil; return d }(), want: "save tree callback is required"},
		{name: "nil save schema", deps: func() Dependencies { d := validDependencies(); d.SaveSchema = nil; return d }(), want: "save schema callback is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(0, tt.deps)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestRun_RejectsUnsupportedMigrationVersion(t *testing.T) {
	deps := validDependencies()
	deps.CurrentSchemaVersion = 5

	err := Run(4, deps)
	if err == nil {
		t.Fatalf("expected error for unsupported migration version")
	}
	if !strings.Contains(err.Error(), "unsupported schema migration version: 4") {
		t.Fatalf("expected unsupported migration version error, got: %v", err)
	}
}
