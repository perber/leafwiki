package backup

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactCredentials(t *testing.T) {
	base := errors.New("boom")
	cases := []struct {
		name string
		err  error
		cfg  Config
		want string
	}{
		{name: "nil", err: nil, cfg: Config{RemoteURL: "https://u:tok@h/r.git"}, want: ""},
		{name: "url with token", err: errors.New(`unexpected requesting "https://u:tok@h/r.git/info/refs" status code: 502`), cfg: Config{RemoteURL: "https://u:tok@h/r.git"}, want: `unexpected requesting "https://u:xxxxx@h/r.git/info/refs" status code: 502`},
		{name: "http password", err: errors.New("auth pw123 rejected"), cfg: Config{RemoteURL: "https://h/r.git", HTTPPassword: "pw123"}, want: "auth xxxxx rejected"},
		{name: "nothing to mask", err: base, cfg: Config{RemoteURL: "git@h:r.git"}, want: "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactCredentials(tc.err, tc.cfg)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if got.Error() != tc.want {
				t.Fatalf("got %q, want %q", got.Error(), tc.want)
			}
			if !errors.Is(got, tc.err) {
				t.Fatal("redacted error no longer wraps the original")
			}
		})
	}
}

// A failing HTTP remote must not leak the token from the remote URL into the
// returned error or the status shown in the UI.
func TestRunBackup_HTTPRemoteError_DoesNotLeakToken(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	const token = "s3cr3t-token"
	repo.cfg.RemoteURL = strings.Replace(srv.URL, "http://", "http://user:"+token+"@", 1) + "/repo.git"
	if err := repo.repo.DeleteRemote("origin"); err != nil {
		t.Fatalf("DeleteRemote: %v", err)
	}

	err := repo.RunBackup()
	if err == nil {
		t.Fatal("RunBackup against a 502 remote succeeded")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("error leaks the token: %v", err)
	}
	if last := repo.Status().LastError; strings.Contains(last, token) || last == "" {
		t.Errorf("LastError = %q, want a non-empty message without the token", last)
	}
}
