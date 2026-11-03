package mcp

import (
	"strings"
	"testing"
)

// ─── patch format (parsePatch / ApplyPatch) ──────────────────────────────────

const patchSampleContent = "keep one\nreplace me\nreplace too\nkeep three\ndelete me\nkeep five"

// TestApplyPatch table-drives the patch semantics: 1-based inclusive ranges
// that replace (or delete, with an empty replacement) lines of the content.
func TestApplyPatch(t *testing.T) {
	tests := []struct {
		name    string
		content string
		ranges  []PatchRange
		want    string
		wantErr string // substring of the error; empty means no error
	}{
		{
			name:    "replace a middle range",
			content: patchSampleContent,
			ranges:  []PatchRange{{Start: 2, End: 3, Replace: "new two\nnew three"}},
			want:    "keep one\nnew two\nnew three\nkeep three\ndelete me\nkeep five",
		},
		{
			name:    "replace the first line",
			content: patchSampleContent,
			ranges:  []PatchRange{{Start: 1, End: 1, Replace: "first now"}},
			want:    "first now\nreplace me\nreplace too\nkeep three\ndelete me\nkeep five",
		},
		{
			name:    "replace through the last line",
			content: patchSampleContent,
			ranges:  []PatchRange{{Start: 5, End: 6, Replace: "the end"}},
			want:    "keep one\nreplace me\nreplace too\nkeep three\nthe end",
		},
		{
			name:    "an empty replacement deletes the range",
			content: patchSampleContent,
			ranges:  []PatchRange{{Start: 5, End: 5, Replace: ""}},
			want:    "keep one\nreplace me\nreplace too\nkeep three\nkeep five",
		},
		{
			name:    "multiple disjoint ranges apply independently",
			content: patchSampleContent,
			ranges: []PatchRange{
				{Start: 2, End: 2, Replace: "second"},
				{Start: 5, End: 5, Replace: ""},
			},
			want: "keep one\nsecond\nreplace too\nkeep three\nkeep five",
		},
		{
			name:    "adjacent ranges are allowed",
			content: "a\nb\nc",
			ranges: []PatchRange{
				{Start: 1, End: 1, Replace: "A"},
				{Start: 2, End: 2, Replace: "B"},
			},
			want: "A\nB\nc",
		},
		{
			name:    "unsorted ranges are applied in order",
			content: "a\nb\nc\nd",
			ranges: []PatchRange{
				{Start: 3, End: 3, Replace: "C"},
				{Start: 1, End: 1, Replace: "A"},
			},
			want: "A\nb\nC\nd",
		},
		{
			name:    "deleting every line leaves an empty page",
			content: "only",
			ranges:  []PatchRange{{Start: 1, End: 1, Replace: ""}},
			want:    "",
		},
		{
			name:    "a range out of range at the start",
			content: "one",
			ranges:  []PatchRange{{Start: 2, End: 2, Replace: "x"}},
			wantErr: "beyond",
		},
		{
			name:    "a range out of range at the end",
			content: "one",
			ranges:  []PatchRange{{Start: 1, End: 2, Replace: "x"}},
			wantErr: "beyond",
		},
		{
			name:    "overlapping ranges are rejected",
			content: "a\nb\nc\nd",
			ranges: []PatchRange{
				{Start: 1, End: 2, Replace: "x"},
				{Start: 2, End: 3, Replace: "y"},
			},
			wantErr: "overlap",
		},
		{
			name:    "a later range nested inside an earlier one is rejected",
			content: "a\nb\nc\nd\ne",
			ranges: []PatchRange{
				{Start: 1, End: 5, Replace: "x"},
				{Start: 2, End: 3, Replace: "y"},
			},
			wantErr: "overlap",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyPatch(tc.content, tc.ranges)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got nil (content %q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want a message containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyPatch error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ApplyPatch = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestApplyPatch_ValidatesRangeShape rejects structurally invalid ranges
// (they are caught by parsePatch before this point, but the helper is the
// second line of defence).
func TestApplyPatch_ValidatesRangeShape(t *testing.T) {
	tests := []struct {
		name   string
		ranges []PatchRange
	}{
		{name: "a zero start", ranges: []PatchRange{{Start: 0, End: 1, Replace: "x"}}},
		{name: "an end before the start", ranges: []PatchRange{{Start: 2, End: 1, Replace: "x"}}},
		{name: "an empty patch", ranges: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ApplyPatch("a\nb", tc.ranges); err == nil {
				t.Fatalf("ApplyPatch(%+v) succeeded, want an error", tc.ranges)
			}
		})
	}
}

// TestParsePatch decodes and validates the wire form of the patch argument.
func TestParsePatch(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []PatchRange
		wantErr bool
	}{
		{
			name: "a valid range array",
			raw:  `[{"start":1,"end":1,"replace":"x"},{"start":3,"end":3,"replace":""}]`,
			want: []PatchRange{{Start: 1, End: 1, Replace: "x"}, {Start: 3, End: 3, Replace: ""}},
		},
		{name: "a non-array value", raw: `"not an array"`, wantErr: true},
		{name: "an object is not an array", raw: `{}`, wantErr: true},
		{name: "an empty array", raw: `[]`, wantErr: true},
		{name: "a missing start defaults to zero and fails", raw: `[{"end":1,"replace":"x"}]`, wantErr: true},
		{name: "an end before the start", raw: `[{"start":2,"end":1,"replace":"x"}]`, wantErr: true},
		{name: "a non-numeric start", raw: `[{"start":"one","end":1,"replace":"x"}]`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePatch(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePatch(%s) succeeded with %+v, want an error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePatch(%s) error: %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parsePatch = %+v, want %+v", got, tc.want)
			}
			for i, r := range got {
				if r != tc.want[i] {
					t.Fatalf("parsePatch[%d] = %+v, want %+v", i, r, tc.want[i])
				}
			}
		})
	}
}
