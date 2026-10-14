package links

import "testing"

func TestResolveURLPath_PageFolderDefault(t *testing.T) {
	if ActiveRelativeLinkMode() != LinkModePageFolder {
		t.Fatalf("expected default mode page-folder, got %s", ActiveRelativeLinkMode())
	}

	cases := []struct{ current, href, want string }{
		{"/docs/guide", "setup", "/docs/guide/setup"},
		{"/docs/guide", "../other", "/docs/other"},
		{"/docs/guide", "./child/page", "/docs/guide/child/page"},
		{"/docs/guide", "/root-page", "/root-page"},
	}
	for _, c := range cases {
		got, err := resolveURLPath(c.current, c.href)
		if err != nil {
			t.Fatalf("resolveURLPath(%q, %q): %v", c.current, c.href, err)
		}
		if got != c.want {
			t.Errorf("resolveURLPath(%q, %q) = %q, want %q", c.current, c.href, got, c.want)
		}
	}
}

func TestResolveURLPath_FilesystemMode(t *testing.T) {
	SetRelativeLinkMode(LinkModeFilesystem)
	t.Cleanup(func() { SetRelativeLinkMode(LinkModePageFolder) })

	cases := []struct{ current, href, want string }{
		// a link to a sibling page/file resolves against the parent dir
		{"/docs/guide", "sibling", "/docs/sibling"},
		{"/docs/guide", "sibling/page", "/docs/sibling/page"},
		// parent-directory targets need one ../ less than page-folder semantics
		{"/docs/nested/guide", "../other", "/docs/other"},
		{"/docs/guide", "./same-dir", "/docs/same-dir"},
		// top-level pages resolve against the root
		{"/top", "x", "/x"},
		{"/top", "../x", "/x"},
		// absolute wiki paths are untouched by the mode
		{"/docs/guide", "/root-page", "/root-page"},
	}
	for _, c := range cases {
		got, err := resolveURLPath(c.current, c.href)
		if err != nil {
			t.Fatalf("resolveURLPath(%q, %q): %v", c.current, c.href, err)
		}
		if got != c.want {
			t.Errorf("resolveURLPath(%q, %q) = %q, want %q", c.current, c.href, got, c.want)
		}
	}
}

func TestSetRelativeLinkMode_UnknownFallsBack(t *testing.T) {
	SetRelativeLinkMode("bogus")
	t.Cleanup(func() { SetRelativeLinkMode(LinkModePageFolder) })

	if ActiveRelativeLinkMode() != LinkModePageFolder {
		t.Fatalf("expected unknown mode to fall back to page-folder, got %s", ActiveRelativeLinkMode())
	}
}
