package dto

import (
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
)

// newTestUserResolver returns a resolver backed by a fresh user store that
// holds one user, alice, and her generated ID.
func newTestUserResolver(t *testing.T) (*auth.UserResolver, string) {
	t.Helper()
	store, err := auth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	users := auth.NewUserService(store)
	alice, err := users.CreateUser("alice", "alice@example.com", "secure", auth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	resolver, err := auth.NewUserResolver(func() *auth.UserService { return users })
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}
	return resolver, alice.ID
}

// The resolved author labels are part of the public wire format; whichever Go
// type carries them, the bytes must stay the same. $ALICE stands for alice's
// generated user ID.
func TestPageDTOs_Marshal_AuthorLabelWireShape(t *testing.T) {
	resolver, aliceID := newTestUserResolver(t)
	node := func(authorID string) *tree.PageNode {
		return &tree.PageNode{
			ID: "p1", Title: "Page", Slug: "page", Kind: tree.NodeKindPage,
			Metadata: tree.PageMetadata{CreatorID: authorID, LastAuthorID: authorID},
		}
	}
	const zeroTime = "0001-01-01T00:00:00Z"
	metadata := func(authorID, labels string) string {
		return `{"createdAt":"` + zeroTime + `","updatedAt":"` + zeroTime + `","creatorId":"` + authorID + `","lastAuthorId":"` + authorID + `"` + labels + `}`
	}
	alice := `{"id":"$ALICE","username":"alice"}`

	tests := []struct {
		name  string
		value func() any
		want  string
	}{
		{
			name:  "tagged page with a known author",
			value: func() any { return ToTaggedPage(node(aliceID), []string{"go"}, "", resolver) },
			want:  `{"id":"p1","title":"Page","path":"page","tags":["go"],"creatorId":"$ALICE","lastAuthorId":"$ALICE","lastAuthor":` + alice + `}`,
		},
		{
			name:  "tagged page with an unknown author omits lastAuthor",
			value: func() any { return ToTaggedPage(node("ghost"), []string{"go"}, "", resolver) },
			want:  `{"id":"p1","title":"Page","path":"page","tags":["go"],"creatorId":"ghost","lastAuthorId":"ghost"}`,
		},
		{
			name:  "tagged page without a resolver omits lastAuthor",
			value: func() any { return ToTaggedPage(node(aliceID), []string{"go"}, "", nil) },
			want:  `{"id":"p1","title":"Page","path":"page","tags":["go"],"creatorId":"$ALICE","lastAuthorId":"$ALICE"}`,
		},
		{
			name:  "property page with a known author",
			value: func() any { return ToPropertyPage(node(aliceID), nil, resolver) },
			want:  `{"id":"p1","title":"Page","path":"page","properties":{},"creatorId":"$ALICE","lastAuthorId":"$ALICE","lastAuthor":` + alice + `}`,
		},
		{
			name:  "property page without a resolver omits lastAuthor",
			value: func() any { return ToPropertyPage(node(aliceID), nil, nil) },
			want:  `{"id":"p1","title":"Page","path":"page","properties":{},"creatorId":"$ALICE","lastAuthorId":"$ALICE"}`,
		},
		{
			name:  "node metadata with a known author",
			value: func() any { return ToAPINode(node(aliceID), "", resolver).Metadata },
			want:  metadata("$ALICE", `,"creator":`+alice+`,"lastAuthor":`+alice),
		},
		{
			name:  "node metadata with an unknown author omits the labels",
			value: func() any { return ToAPINode(node("ghost"), "", resolver).Metadata },
			want:  metadata("ghost", ""),
		},
		{
			name:  "node metadata without an author omits the labels",
			value: func() any { return ToAPINode(node(""), "", resolver).Metadata },
			want:  metadata("", ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, tt.value(), strings.ReplaceAll(tt.want, "$ALICE", aliceID))
		})
	}
}
