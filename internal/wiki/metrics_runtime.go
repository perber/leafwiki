package wiki

import (
	"github.com/perber/wiki/internal/core/auth"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
)

// runtimeStatsSource adapts the wiki's tree and user services to the
// httpmetrics.RuntimeStatsSource interface behind the scrape-time
// leafwiki_pages / leafwiki_users gauges. It holds the *Wiki rather than the
// concrete services so UserStats always reads the current, restore-swap-aware
// UserService (see Wiki.UserService).
type runtimeStatsSource struct {
	w *Wiki
}

func (s runtimeStatsSource) PageStats() (httpmetrics.PageStats, error) {
	pages, sections := s.w.tree.NodeCounts()
	return httpmetrics.PageStats{Pages: pages, Sections: sections}, nil
}

func (s runtimeStatsSource) UserStats() (httpmetrics.UserStats, error) {
	users, err := s.w.UserService().GetUsers()
	if err != nil {
		return httpmetrics.UserStats{}, err
	}

	// Seed every known role so its series stays present at zero.
	stats := httpmetrics.UserStats{
		ByRole: map[string]int{
			auth.RoleAdmin:  0,
			auth.RoleEditor: 0,
			auth.RoleViewer: 0,
		},
	}
	for _, u := range users {
		stats.ByRole[u.Role]++
		if u.TOTPEnabled {
			stats.TOTPEnabled++
		}
		if u.MustSetPassword {
			stats.PendingInvite++
		}
	}
	return stats, nil
}
