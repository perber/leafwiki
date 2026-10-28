package seo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki/seo"
)

// fakeFirstPageReader returns a fixed first page (nil = empty wiki) or error
// and counts its reads.
type fakeFirstPageReader struct {
	page  *seo.FirstPage
	err   error
	reads int
}

func (f *fakeFirstPageReader) FirstPage(context.Context) (*seo.FirstPage, error) {
	f.reads++
	return f.page, f.err
}

func firstPage(routePath string) *fakeFirstPageReader {
	return &fakeFirstPageReader{page: &seo.FirstPage{RoutePath: routePath}}
}

func TestGetRootTargetUseCase_FirstPage_ReturnsLocation(t *testing.T) {
	tests := []struct {
		name      string
		basePath  string
		routePath string
		want      string
	}{
		{name: "no base path", routePath: "docs", want: "/docs"},
		{name: "base path", basePath: "/wiki", routePath: "docs", want: "/wiki/docs"},
		{name: "nested base path", basePath: "/a/b", routePath: "docs", want: "/a/b/docs"},
		{name: "slash base path is the root", basePath: "/", routePath: "docs", want: "/docs"},
		{name: "trailing slash on base path", basePath: "/wiki/", routePath: "docs", want: "/wiki/docs"},
		{name: "nested route path keeps its slashes", routePath: "docs/setup", want: "/docs/setup"},
		{name: "base path is escaped", basePath: "/my wiki", routePath: "docs", want: "/my%20wiki/docs"},
		{name: "space in segment", routePath: "a b", want: "/a%20b"},
		{name: "query and fragment characters in segment", routePath: "a?b#c", want: "/a%3Fb%23c"},
		{name: "percent in segment", routePath: "x%2Fy", want: "/x%252Fy"},
		{name: "CR and LF in segment", routePath: "a\r\nb", want: "/a%0D%0Ab"},
		{name: "backslash in first segment", routePath: `\evil.example`, want: "/%5Cevil.example"},
		{name: "unicode segment", routePath: "größe", want: "/gr%C3%B6%C3%9Fe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := seo.NewGetRootTargetUseCase(firstPage(tc.routePath))

			out, err := uc.Execute(context.Background(), seo.GetRootTargetInput{BasePath: tc.basePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if out.Location != tc.want {
				t.Errorf("Location = %q, want %q", out.Location, tc.want)
			}
			if strings.HasPrefix(out.Location, "//") || strings.ContainsAny(out.Location, "\r\n") {
				t.Errorf("Location %q must not start with // or contain CR/LF", out.Location)
			}
		})
	}
}

func TestGetRootTargetUseCase_EmptyWiki_ReturnsNoLocation(t *testing.T) {
	uc := seo.NewGetRootTargetUseCase(&fakeFirstPageReader{})

	out, err := uc.Execute(context.Background(), seo.GetRootTargetInput{BasePath: "/wiki"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out.Location != "" {
		t.Errorf("Location = %q, want empty", out.Location)
	}
}

func TestGetRootTargetUseCase_UnusableTarget_ReturnsError(t *testing.T) {
	readErr := errors.New("disk on fire")
	tests := []struct {
		name     string
		reader   *fakeFirstPageReader
		basePath string
		wantErr  error
	}{
		{name: "reader error is passed through", reader: &fakeFirstPageReader{err: readErr}, wantErr: readErr},
		{name: "empty route path", reader: firstPage("")},
		{name: "empty segment", reader: firstPage("docs//setup")},
		{name: "leading slash in route path", reader: firstPage("/docs")},
		{name: "protocol-relative base path", reader: firstPage("docs"), basePath: "//evil.example"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := seo.NewGetRootTargetUseCase(tc.reader)

			out, err := uc.Execute(context.Background(), seo.GetRootTargetInput{BasePath: tc.basePath})
			if err == nil {
				t.Fatalf("expected error, got %+v", out)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
