package backup

import "testing"

func TestClassifyRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   RemoteTransport
	}{
		{"", TransportUnknown},
		{"   ", TransportUnknown},
		{"https://github.com/user/repo.git", TransportHTTP},
		{"http://gitea.internal/user/repo.git", TransportHTTP},
		{"HTTPS://github.com/user/repo.git", TransportHTTP},
		{"  https://github.com/user/repo.git  ", TransportHTTP},
		{"git@github.com:user/repo.git", TransportSSH},
		{"ssh://git@github.com/user/repo.git", TransportSSH},
		{"SSH://git@github.com/user/repo.git", TransportSSH},
		{"file:///tmp/bare", TransportUnknown},
		{"/tmp/bare", TransportUnknown},
		{"not a url", TransportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.remote, func(t *testing.T) {
			if got := ClassifyRemote(tc.remote); got != tc.want {
				t.Fatalf("ClassifyRemote(%q) = %q, want %q", tc.remote, got, tc.want)
			}
		})
	}
}
