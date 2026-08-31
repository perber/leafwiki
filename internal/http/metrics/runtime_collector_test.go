package metrics

import (
	"errors"
	"strings"
	"testing"
)

type fakeRuntimeSource struct {
	pages    PageStats
	pagesErr error
	users    UserStats
	usersErr error
}

func (f fakeRuntimeSource) PageStats() (PageStats, error) { return f.pages, f.pagesErr }
func (f fakeRuntimeSource) UserStats() (UserStats, error) { return f.users, f.usersErr }

func TestHTTPMetrics_RegisterRuntimeStats_ExportsContentAndUserGauges(t *testing.T) {
	m := NewHTTPMetrics("test")
	m.RegisterRuntimeStats(fakeRuntimeSource{
		pages: PageStats{Pages: 42, Sections: 7},
		users: UserStats{
			ByRole:        map[string]int{"admin": 1, "editor": 3, "viewer": 0},
			TOTPEnabled:   2,
			PendingInvite: 1,
		},
	})

	body := getMetricsBody(t, m.HTTPHandler())

	for _, want := range []string{
		`leafwiki_pages{kind="page"} 42`,
		`leafwiki_pages{kind="section"} 7`,
		`leafwiki_users{role="admin"} 1`,
		`leafwiki_users{role="editor"} 3`,
		`leafwiki_users{role="viewer"} 0`,
		`leafwiki_users_with_totp 2`,
		`leafwiki_users_pending_invite 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected metrics output to contain %q, got:\n%s", want, body)
		}
	}
}

func TestHTTPMetrics_RegisterRuntimeStats_SkipsGaugesWhenSourceErrors(t *testing.T) {
	m := NewHTTPMetrics("test")
	m.RegisterRuntimeStats(fakeRuntimeSource{
		pagesErr: errors.New("tree not loaded"),
		usersErr: errors.New("user store unavailable"),
	})

	// A source error must not break the whole endpoint: /metrics still
	// serves 200 with the other metrics, just without the runtime gauges.
	body := getMetricsBody(t, m.HTTPHandler())
	if !strings.Contains(body, "leafwiki_build_info") {
		t.Fatalf("expected the rest of /metrics to keep working, got:\n%s", body)
	}
	if strings.Contains(body, "leafwiki_pages{") || strings.Contains(body, "leafwiki_users{") {
		t.Errorf("expected no runtime gauge samples when source errors, got:\n%s", body)
	}
}

func TestHTTPMetrics_RegisterRuntimeStats_NilReceiverIsNoop(t *testing.T) {
	var m *HTTPMetrics
	m.RegisterRuntimeStats(fakeRuntimeSource{}) // must not panic
}
