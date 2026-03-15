package tree

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"time"
)

// PageMetadata holds simple metadata for a page.
type PageMetadata struct {
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatorID    string    `json:"creatorId"`
	LastAuthorID string    `json:"lastAuthorId"`
}

type NodeKind string

const (
	NodeKindPage    NodeKind = "page"
	NodeKindSection NodeKind = "section"
)

type NodeIssueCode string

const (
	NodeIssueMissingIndexMD NodeIssueCode = "missing_index_md"
	NodeIssueMissingID      NodeIssueCode = "missing_leafwiki_id"
	NodeIssueDuplicateID    NodeIssueCode = "duplicate_leafwiki_id"
	NodeIssueInvalidOrder   NodeIssueCode = "invalid_order_json"
)

type NodeIssue struct {
	Code    NodeIssueCode `json:"code"`
	Message string        `json:"message"`
}

// This is an in-memory projection reconstructed from the filesystem.
// Parent/Children/Kind are derived, not persisted truth.
type PageNode struct {
	ID       string       `json:"id"`       // Unique identifier for the entry
	Title    string       `json:"title"`    // Title is the name of the entry
	Slug     string       `json:"slug"`     // Slug is the path of the entry
	Metadata PageMetadata `json:"metadata"` // Metadata holds metadata about the page

	// Derived fields (not persisted)
	Parent   *PageNode   `json:"-"`
	Children []*PageNode `json:"children,omitempty"`
	Kind     NodeKind    `json:"kind"`

	// Visible repair/drift markers for app/browser
	RepairNeeded bool        `json:"repairNeeded"`
	Issues       []NodeIssue `json:"issues,omitempty"`
}

func (p *PageNode) Hash() string {
	sum := p.hashSum(true) // includeMetadata = true
	return hex.EncodeToString(sum[:])
}

func (p *PageNode) HasChildren() bool {
	return len(p.Children) > 0
}

func (p *PageNode) ChildAlreadyExists(slug string) bool {
	for _, child := range p.Children {
		if child.Slug == slug {
			return true
		}
	}
	return false
}

func (p *PageNode) HasDescendant(childID string, recursive bool) bool {
	for _, child := range p.Children {
		if child.ID == childID {
			return true
		}
		if recursive && child.HasDescendant(childID, recursive) {
			return true
		}
	}
	return false
}

func (p *PageNode) CalculatePath() string {
	// Calculate the path of the entry
	// The path is the slug of the entry and its parent's path
	if p.Parent == nil {
		if p.Slug == "" || p.Slug == "root" {
			return ""
		}
		return p.Slug
	}
	return p.Parent.CalculatePath() + "/" + p.Slug
}

func (p *PageNode) hashSum(includeMetadata bool) [32]byte {
	h := sha256.New()

	// depth-first, deterministic
	// Write directly to hash to avoid buffering entire tree in memory
	p.writeHashPayload(h, includeMetadata)

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (p *PageNode) writeHashPayload(w io.Writer, includeMetadata bool) {
	// Node fields (parent excluded)
	writeString(w, "id")
	writeString(w, p.ID)
	writeString(w, "title")
	writeString(w, p.Title)
	writeString(w, "slug")
	writeString(w, p.Slug)
	writeString(w, "kind")
	writeString(w, string(p.Kind))

	if p.RepairNeeded {
		writeInt64(w, 1)
	} else {
		writeInt64(w, 0)
	}

	if includeMetadata {
		writeString(w, "meta.createdAt")
		writeTime(w, p.Metadata.CreatedAt)
		writeString(w, "meta.updatedAt")
		writeTime(w, p.Metadata.UpdatedAt)
		writeString(w, "meta.creatorId")
		writeString(w, p.Metadata.CreatorID)
		writeString(w, "meta.lastAuthorId")
		writeString(w, p.Metadata.LastAuthorID)
	}

	writeString(w, "children.count")
	writeInt64(w, int64(len(p.Children)))
	for _, ch := range p.Children {
		if ch == nil {
			writeString(w, "child.nil")
			continue
		}
		// Separator for safety
		writeString(w, "child.begin")
		ch.writeHashPayload(w, includeMetadata)
		writeString(w, "child.end")
	}
}

func writeString(w io.Writer, s string) {
	// length-prefixed string (uint32 len + bytes)
	_ = binary.Write(w, binary.BigEndian, uint32(len(s)))
	_, _ = io.WriteString(w, s)
}

func writeInt64(w io.Writer, v int64) {
	_ = binary.Write(w, binary.BigEndian, v)
}

func writeTime(w io.Writer, t time.Time) {
	// stable: UnixNano in UTC (Zero => 0)
	if t.IsZero() {
		writeInt64(w, 0)
		return
	}
	writeInt64(w, t.UTC().UnixNano())
}
