package tree

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
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

// PageNode represents a single node in the tree
// It has an ID, a parent, a path, and children
// The ID is a unique identifier for the entry
type PageNode struct {
	ID       string      `json:"id"`       // Unique identifier for the entry
	Title    string      `json:"title"`    // Title is the name of the entry
	Slug     string      `json:"slug"`     // Slug is the path of the entry
	Children []*PageNode `json:"children"` // Children are the children of the entry
	Position int         `json:"position"` // Position is the position of the entry
	Parent   *PageNode   `json:"-"`

	Kind     NodeKind     `json:"kind"`     // Kind is the kind of the node (page or folder)
	Metadata PageMetadata `json:"metadata"` // Metadata holds metadata about the page
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

func (p *PageNode) IsChildOf(childID string, recursive bool) bool {
	for _, child := range p.Children {
		if child.ID == childID {
			return true
		}
		if recursive && child.IsChildOf(childID, recursive) {
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

// Hash returns a deterministic hash of the node and all descendants.
// Parent is intentionally ignored to avoid cycles.
func (p *PageNode) Hash() string {
	sum := p.hashSum(true) // includeMetadata = true
	return hex.EncodeToString(sum[:])
}

func (p *PageNode) hashSum(includeMetadata bool) [32]byte {
	h := sha256.New()
	var buf bytes.Buffer

	// depth-first, deterministic
	p.writeHashPayload(&buf, includeMetadata)
	_, _ = h.Write(buf.Bytes())

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (p *PageNode) writeHashPayload(buf *bytes.Buffer, includeMetadata bool) {
	// Node fields (parent excluded)
	writeString(buf, "id")
	writeString(buf, p.ID)
	writeString(buf, "title")
	writeString(buf, p.Title)
	writeString(buf, "slug")
	writeString(buf, p.Slug)
	writeString(buf, "kind")
	writeString(buf, string(p.Kind))
	writeString(buf, "position")
	writeInt64(buf, int64(p.Position))

	if includeMetadata {
		writeString(buf, "meta.createdAt")
		writeTime(buf, p.Metadata.CreatedAt)
		writeString(buf, "meta.updatedAt")
		writeTime(buf, p.Metadata.UpdatedAt)
		writeString(buf, "meta.creatorId")
		writeString(buf, p.Metadata.CreatorID)
		writeString(buf, "meta.lastAuthorId")
		writeString(buf, p.Metadata.LastAuthorID)
	}

	// Children: enforce stable order (Position, then ID as tie-breaker)
	children := make([]*PageNode, 0, len(p.Children))
	children = append(children, p.Children...)

	sort.SliceStable(children, func(i, j int) bool {
		if children[i] == nil || children[j] == nil {
			return children[j] != nil // nils last
		}
		if children[i].Position != children[j].Position {
			return children[i].Position < children[j].Position
		}
		return children[i].ID < children[j].ID
	})

	writeString(buf, "children.count")
	writeInt64(buf, int64(len(children)))

	for _, ch := range children {
		if ch == nil {
			writeString(buf, "child.nil")
			continue
		}
		// Separator for safety
		writeString(buf, "child.begin")
		ch.writeHashPayload(buf, includeMetadata)
		writeString(buf, "child.end")
	}
}

func writeString(buf *bytes.Buffer, s string) {
	// length-prefixed string (uint32 len + bytes)
	_ = binary.Write(buf, binary.BigEndian, uint32(len(s)))
	_, _ = buf.WriteString(s)
}

func writeInt64(buf *bytes.Buffer, v int64) {
	_ = binary.Write(buf, binary.BigEndian, v)
}

func writeTime(buf *bytes.Buffer, t time.Time) {
	// stabil: UnixNano in UTC (Zero => 0)
	if t.IsZero() {
		writeInt64(buf, 0)
		return
	}
	writeInt64(buf, t.UTC().UnixNano())
}
