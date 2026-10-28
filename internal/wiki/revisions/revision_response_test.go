package revisions

import (
	"encoding/json"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/revision"
)

// newTestUserResolver returns a resolver backed by a fresh user store that
// holds one user, alice, and her generated ID.
func newTestUserResolver(t *testing.T) (*coreauth.UserResolver, string) {
	t.Helper()
	store, err := coreauth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	users := coreauth.NewUserService(store)
	alice, err := users.CreateUser("alice", "alice@example.com", "secure", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	resolver, err := coreauth.NewUserResolver(func() *coreauth.UserService { return users })
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}
	return resolver, alice.ID
}

// The revision author label is part of the public wire format: an object with
// exactly id and username when the author resolves, no key otherwise.
func TestRevisionResponse_Marshal_AuthorWireShape(t *testing.T) {
	resolver, aliceID := newTestUserResolver(t)

	tests := []struct {
		name     string
		authorID string
		resolver *coreauth.UserResolver
		want     string // raw JSON of "author"; empty means the key is absent
	}{
		{name: "known author", authorID: aliceID, resolver: resolver, want: `{"id":"` + aliceID + `","username":"alice"}`},
		{name: "unknown author is omitted", authorID: "ghost", resolver: resolver},
		{name: "empty author is omitted", authorID: "", resolver: resolver},
		{name: "no resolver omits the author", authorID: aliceID, resolver: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := toRevisionResponse(&revision.Revision{ID: "r1", PageID: "p1", AuthorID: tt.authorID}, tt.resolver)
			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatalf("Unmarshal %s: %v", data, err)
			}
			if got := string(fields["authorId"]); got != `"`+tt.authorID+`"` {
				t.Errorf("authorId = %s, want %q", got, tt.authorID)
			}
			got, present := fields["author"]
			switch {
			case tt.want == "" && present:
				t.Errorf("author = %s, want no author key", got)
			case tt.want != "" && string(got) != tt.want:
				t.Errorf("author = %s, want %s", got, tt.want)
			}
		})
	}
}
