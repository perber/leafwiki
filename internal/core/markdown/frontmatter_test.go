package markdown

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFM   string
		wantBody string
		wantHas  bool
	}{
		{
			name:     "no frontmatter",
			input:    "# Hello\nWorld\n",
			wantFM:   "",
			wantBody: "# Hello\nWorld\n",
			wantHas:  false,
		},
		{
			name:     "simple frontmatter",
			input:    "---\nleafwiki_id: abc123\n---\n# Title\n",
			wantFM:   "leafwiki_id: abc123",
			wantBody: "# Title\n",
			wantHas:  true,
		},
		{
			name:     "frontmatter with blank line",
			input:    "---\nleafwiki_id: abc123\n\n---\nBody\n",
			wantFM:   "leafwiki_id: abc123\n",
			wantBody: "Body\n",
			wantHas:  true,
		},
		{
			name:     "frontmatter with comments",
			input:    "---\n# comment\nleafwiki_id: abc123\n---\nBody\n",
			wantFM:   "# comment\nleafwiki_id: abc123",
			wantBody: "Body\n",
			wantHas:  true,
		},
		{
			name:     "only separator at top (no YAML)",
			input:    "---\nHello\nWorld\n---\nBody\n",
			wantFM:   "",
			wantBody: "---\nHello\nWorld\n---\nBody\n",
			wantHas:  false,
		},
		{
			name:     "horizontal rule later in document",
			input:    "# Title\n\n---\n\nText\n",
			wantFM:   "",
			wantBody: "# Title\n\n---\n\nText\n",
			wantHas:  false,
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\nleafwiki_id: abc123\nBody\n",
			wantFM:   "",
			wantBody: "---\nleafwiki_id: abc123\nBody\n",
			wantHas:  false,
		},
		{
			name:     "empty frontmatter block",
			input:    "---\n---\nBody\n",
			wantFM:   "",
			wantBody: "---\n---\nBody\n",
			wantHas:  false,
		},
		{
			name:     "frontmatter with windows line endings",
			input:    "---\r\nleafwiki_id: abc123\r\n---\r\nBody\r\n",
			wantFM:   "leafwiki_id: abc123",
			wantBody: "Body\n",
			wantHas:  true,
		},
		{
			name:     "frontmatter with BOM",
			input:    "\ufeff---\nleafwiki_id: abc123\n---\nBody\n",
			wantFM:   "leafwiki_id: abc123",
			wantBody: "Body\n",
			wantHas:  true,
		},
		{
			name:     "yaml but no key colon (treated as no frontmatter)",
			input:    "---\n- item1\n- item2\n---\nBody\n",
			wantFM:   "",
			wantBody: "---\n- item1\n- item2\n---\nBody\n",
			wantHas:  false,
		},
		{
			name:     "markdown separator block with smiley is not frontmatter",
			input:    "---\n__Advertisement :)__\n---\nBody\n",
			wantFM:   "",
			wantBody: "---\n__Advertisement :)__\n---\nBody\n",
			wantHas:  false,
		},
		{
			name:     "reference-style link definition is not frontmatter",
			input:    "---\n[id]: https://example.com/demo\n---\nBody\n",
			wantFM:   "",
			wantBody: "---\n[id]: https://example.com/demo\n---\nBody\n",
			wantHas:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, has := splitFrontmatter(tt.input)

			if has != tt.wantHas {
				t.Fatalf("has = %v, want %v", has, tt.wantHas)
			}
			if fm != tt.wantFM {
				t.Fatalf("frontmatter = %q, want %q", fm, tt.wantFM)
			}
			if body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantFM      Frontmatter
		wantBody    string
		wantHas     bool
		wantErr     bool
		wantErrType error
	}{
		{
			name:     "no frontmatter",
			input:    "# Hello\nWorld\n",
			wantFM:   Frontmatter{},
			wantBody: "# Hello\nWorld\n",
			wantHas:  false,
			wantErr:  false,
		},
		{
			name:  "valid frontmatter with ID only",
			input: "---\nleafwiki_id: abc123\n---\n# Title\nContent",
			wantFM: Frontmatter{
				LeafWikiID: "abc123",
			},
			wantBody: "# Title\nContent",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "valid frontmatter with title only",
			input: "---\nleafwiki_title: My Title\n---\n# Title\nContent",
			wantFM: Frontmatter{
				LeafWikiTitle: "My Title",
			},
			wantBody: "# Title\nContent",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "title alias is mapped and preserved",
			input: "---\ntitle: My Title\n---\n# Title\nContent",
			wantFM: Frontmatter{
				LeafWikiTitle: "My Title",
				ExtraFields: map[string]interface{}{
					"title": "My Title",
				},
			},
			wantBody: "# Title\nContent",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "valid frontmatter with both ID and title",
			input: "---\nleafwiki_id: abc123\nleafwiki_title: My Title\n---\n# Title\nContent",
			wantFM: Frontmatter{
				LeafWikiID:    "abc123",
				LeafWikiTitle: "My Title",
			},
			wantBody: "# Title\nContent",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "valid frontmatter with leafwiki metadata",
			input: "---\nleafwiki_id: abc123\nleafwiki_created_at: 2026-03-21T10:15:30Z\nleafwiki_updated_at: 2026-03-21T11:16:31Z\nleafwiki_creator_id: alice\nleafwiki_last_author_id: bob\n---\nBody",
			wantFM: Frontmatter{
				LeafWikiID:           "abc123",
				LeafWikiCreatedAt:    "2026-03-21T10:15:30Z",
				LeafWikiUpdatedAt:    "2026-03-21T11:16:31Z",
				LeafWikiCreatorID:    "alice",
				LeafWikiLastAuthorID: "bob",
			},
			wantBody: "Body",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "unknown fields are preserved",
			input: "---\nkey: value\n---\nBody",
			wantFM: Frontmatter{
				ExtraFields: map[string]interface{}{
					"key": "value",
				},
			},
			wantBody: "Body",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:        "invalid YAML in frontmatter",
			input:       "---\nleafwiki_id: [invalid: yaml: structure\n---\nBody",
			wantFM:      Frontmatter{},
			wantBody:    "---\nleafwiki_id: [invalid: yaml: structure\n---\nBody",
			wantHas:     true,
			wantErr:     true,
			wantErrType: ErrFrontmatterParse,
		},
		{
			name:        "malformed YAML - unclosed brackets",
			input:       "---\nleafwiki_id: {unclosed\n---\nBody",
			wantFM:      Frontmatter{},
			wantBody:    "---\nleafwiki_id: {unclosed\n---\nBody",
			wantHas:     true,
			wantErr:     true,
			wantErrType: ErrFrontmatterParse,
		},
		{
			name:  "frontmatter with extra fields",
			input: "---\nleafwiki_id: abc123\nextra_field: ignored\n---\nBody",
			wantFM: Frontmatter{
				LeafWikiID: "abc123",
				ExtraFields: map[string]interface{}{
					"extra_field": "ignored",
				},
			},
			wantBody: "Body",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "template placeholder scalar is treated as string",
			input: "---\nDatum: {{date}}\n---\nBody",
			wantFM: Frontmatter{
				ExtraFields: map[string]interface{}{
					"Datum": "{{date}}",
				},
			},
			wantBody: "Body",
			wantHas:  true,
			wantErr:  false,
		},
		{
			name:  "frontmatter with whitespace in values",
			input: "---\nleafwiki_id: \"  abc123  \"\nleafwiki_title: \"  My Title  \"\n---\nBody",
			wantFM: Frontmatter{
				LeafWikiID:    "abc123",
				LeafWikiTitle: "  My Title  ",
			},
			wantBody: "Body",
			wantHas:  true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, has, err := ParseFrontmatter(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.wantErrType != nil {
				if !errors.Is(err, tt.wantErrType) {
					t.Fatalf("ParseFrontmatter() error = %v, want error type %v", err, tt.wantErrType)
				}
			}

			if has != tt.wantHas {
				t.Fatalf("has = %v, want %v", has, tt.wantHas)
			}

			if !reflect.DeepEqual(fm, tt.wantFM) {
				t.Fatalf("frontmatter = %+v, want %+v", fm, tt.wantFM)
			}

			if body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestBuildMarkdownWithFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		fm      Frontmatter
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "empty frontmatter struct",
			fm:   Frontmatter{},
			body: "# Title\nContent",
			want: "# Title\nContent",
		},
		{
			name: "frontmatter with empty ID",
			fm: Frontmatter{
				LeafWikiID: "",
			},
			body: "# Title\nContent",
			want: "# Title\nContent",
		},
		{
			name: "frontmatter with whitespace-only ID",
			fm: Frontmatter{
				LeafWikiID: "   ",
			},
			body: "# Title\nContent",
			want: "# Title\nContent",
		},
		{
			name: "frontmatter with ID only",
			fm: Frontmatter{
				LeafWikiID: "abc123",
			},
			body: "# Title\nContent",
			want: "---\nleafwiki_id: abc123\n---\n# Title\nContent",
		},
		{
			name: "frontmatter with title only",
			fm: Frontmatter{
				LeafWikiTitle: "My Title",
			},
			body: "# Title\nContent",
			want: "# Title\nContent",
		},
		{
			name: "frontmatter with both ID and title",
			fm: Frontmatter{
				LeafWikiID:    "abc123",
				LeafWikiTitle: "My Title",
			},
			body: "# Title\nContent",
			want: "---\nleafwiki_id: abc123\nleafwiki_title: My Title\n---\n# Title\nContent",
		},
		{
			name: "frontmatter with metadata fields",
			fm: Frontmatter{
				LeafWikiID:           "abc123",
				LeafWikiTitle:        "My Title",
				LeafWikiCreatedAt:    "2026-03-21T10:15:30Z",
				LeafWikiUpdatedAt:    "2026-03-21T11:16:31Z",
				LeafWikiCreatorID:    "alice",
				LeafWikiLastAuthorID: "bob",
			},
			body: "Content",
			want: "---\nleafwiki_id: abc123\nleafwiki_title: My Title\nleafwiki_created_at: \"2026-03-21T10:15:30Z\"\nleafwiki_updated_at: \"2026-03-21T11:16:31Z\"\nleafwiki_creator_id: alice\nleafwiki_last_author_id: bob\n---\nContent",
		},
		{
			name: "frontmatter preserves unknown fields",
			fm: Frontmatter{
				LeafWikiID:    "abc123",
				LeafWikiTitle: "My Title",
				ExtraFields: map[string]interface{}{
					"custom_key": "keep-me",
				},
			},
			body: "Content",
			want: "---\ncustom_key: keep-me\nleafwiki_id: abc123\nleafwiki_title: My Title\n---\nContent",
		},
		{
			name: "empty body",
			fm: Frontmatter{
				LeafWikiID: "abc123",
			},
			body: "",
			want: "---\nleafwiki_id: abc123\n---\n",
		},
		{
			name: "body with newlines",
			fm: Frontmatter{
				LeafWikiID: "abc123",
			},
			body: "# Title\n\nParagraph 1\n\nParagraph 2\n",
			want: "---\nleafwiki_id: abc123\n---\n# Title\n\nParagraph 1\n\nParagraph 2\n",
		},
		{
			name: "frontmatter with special characters in values",
			fm: Frontmatter{
				LeafWikiID:    "abc-123_xyz",
				LeafWikiTitle: "Title: With Special & Characters",
			},
			body: "Content",
			want: "---\nleafwiki_id: abc-123_xyz\nleafwiki_title: 'Title: With Special & Characters'\n---\nContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildMarkdownWithFrontmatter(tt.fm, tt.body)

			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildMarkdownWithFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Fatalf("BuildMarkdownWithFrontmatter() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatterAndBuildRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBody string
	}{
		{
			name:     "no frontmatter",
			input:    "# Title\nContent",
			wantBody: "# Title\nContent",
		},
		{
			name:     "with ID only",
			input:    "---\nleafwiki_id: abc123\n---\n# Title\nContent",
			wantBody: "# Title\nContent",
		},
		{
			name:     "with ID and title",
			input:    "---\nleafwiki_id: abc123\nleafwiki_title: My Title\n---\n# Title\nContent",
			wantBody: "# Title\nContent",
		},
		{
			name:     "with unknown fields",
			input:    "---\nleafwiki_id: abc123\ncustom_key: keep-me\n---\n# Title\nContent",
			wantBody: "# Title\nContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, has, err := ParseFrontmatter(tt.input)
			if err != nil {
				t.Fatalf("ParseFrontmatter() error = %v", err)
			}

			if body != tt.wantBody {
				t.Fatalf("body after parse = %q, want %q", body, tt.wantBody)
			}

			rebuilt, err := BuildMarkdownWithFrontmatter(fm, body)
			if err != nil {
				t.Fatalf("BuildMarkdownWithFrontmatter() error = %v", err)
			}

			fm2, body2, has2, err := ParseFrontmatter(rebuilt)
			if err != nil {
				t.Fatalf("ParseFrontmatter() second parse error = %v", err)
			}

			if has != has2 {
				t.Fatalf("has flag changed: first=%v, second=%v", has, has2)
			}

			if !reflect.DeepEqual(fm, fm2) {
				t.Fatalf("frontmatter changed: first=%+v, second=%+v", fm, fm2)
			}

			if body != body2 {
				t.Fatalf("body changed: first=%q, second=%q", body, body2)
			}
		})
	}
}

func TestParseFrontmatter_ScalarLeafWikiValuesArePreserved(t *testing.T) {
	fm, body, has, err := ParseFrontmatter(`---
leafwiki_id: 123
leafwiki_title: true
---
Body`)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if fm.LeafWikiID != "123" {
		t.Fatalf("expected numeric id to be preserved, got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "true" {
		t.Fatalf("expected bool title to be preserved, got %q", fm.LeafWikiTitle)
	}
	if body != "Body" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestBuildMarkdownWithFrontmatter_SortsExtraFieldsDeterministically(t *testing.T) {
	fm := Frontmatter{
		LeafWikiID:    "abc123",
		LeafWikiTitle: "My Title",
		ExtraFields: map[string]interface{}{
			"z_key": "last",
			"a_key": "first",
		},
	}

	got, err := BuildMarkdownWithFrontmatter(fm, "Content")
	if err != nil {
		t.Fatalf("BuildMarkdownWithFrontmatter() error = %v", err)
	}

	want := `---
a_key: first
z_key: last
leafwiki_id: abc123
leafwiki_title: My Title
---
Content`
	if got != want {
		t.Fatalf("BuildMarkdownWithFrontmatter() =\n%q\nwant:\n%q", got, want)
	}
}

func TestBuildMarkdownWithExtraFrontmatter_SortsExtraFieldsDeterministically(t *testing.T) {
	got, err := BuildMarkdownWithExtraFrontmatter(map[string]interface{}{
		"z_key": "last",
		"a_key": "first",
	}, "Content")
	if err != nil {
		t.Fatalf("BuildMarkdownWithExtraFrontmatter() error = %v", err)
	}

	want := `---
a_key: first
z_key: last
---
Content`
	if got != want {
		t.Fatalf("BuildMarkdownWithExtraFrontmatter() =\n%q\nwant:\n%q", got, want)
	}
}

func TestFrontmatter_MetadataRoundtripRFC3339(t *testing.T) {
	createdAt := time.Date(2026, time.March, 21, 10, 15, 30, 0, time.UTC).Format(time.RFC3339)
	updatedAt := time.Date(2026, time.March, 21, 11, 16, 31, 0, time.UTC).Format(time.RFC3339)

	input := Frontmatter{
		LeafWikiID:           "abc123",
		LeafWikiTitle:        "My Title",
		LeafWikiCreatedAt:    createdAt,
		LeafWikiUpdatedAt:    updatedAt,
		LeafWikiCreatorID:    "alice",
		LeafWikiLastAuthorID: "bob",
	}

	raw, err := BuildMarkdownWithFrontmatter(input, "Body")
	if err != nil {
		t.Fatalf("BuildMarkdownWithFrontmatter() error = %v", err)
	}

	fm, body, has, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter() error = %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if body != "Body" {
		t.Fatalf("unexpected body %q", body)
	}
	if !reflect.DeepEqual(fm, input) {
		t.Fatalf("frontmatter changed: got %+v want %+v", fm, input)
	}
}
