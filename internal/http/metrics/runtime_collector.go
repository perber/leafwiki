package metrics

import (
	"log/slog"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
)

// PageStats is a point-in-time snapshot of how much content the wiki holds.
type PageStats struct {
	Pages    int
	Sections int
}

// UserStats is a point-in-time snapshot of the user base. ByRole should be
// pre-seeded by the caller with every role it wants exported (zero included),
// so a role's series does not vanish when its count drops to zero.
type UserStats struct {
	ByRole        map[string]int
	TOTPEnabled   int
	PendingInvite int
}

// RuntimeStatsSource supplies the values behind the scrape-time content and
// user gauges. Both methods are called on every /metrics scrape, so they must
// be cheap; an error makes that scrape skip the affected gauges.
type RuntimeStatsSource interface {
	PageStats() (PageStats, error)
	UserStats() (UserStats, error)
}

// runtimeCollector is a prometheus.Collector that reads its values from a
// RuntimeStatsSource at scrape time rather than tracking them incrementally.
// Counts like "how many pages" and "how many users" have no natural event to
// hook a counter onto and are always a current total, so a pull-based gauge
// collector is the honest representation.
type runtimeCollector struct {
	src RuntimeStatsSource
	log *slog.Logger

	pages         *prometheus.Desc
	users         *prometheus.Desc
	usersWithTOTP *prometheus.Desc
	usersPending  *prometheus.Desc
}

func newRuntimeCollector(src RuntimeStatsSource) *runtimeCollector {
	return &runtimeCollector{
		src: src,
		log: slog.Default().With("component", "metrics.runtime"),
		pages: prometheus.NewDesc(
			"leafwiki_pages",
			"Current number of nodes in the page tree by kind (page or section).",
			[]string{"kind"}, nil,
		),
		users: prometheus.NewDesc(
			"leafwiki_users",
			"Current number of user accounts by role.",
			[]string{"role"}, nil,
		),
		usersWithTOTP: prometheus.NewDesc(
			"leafwiki_users_with_totp",
			"Current number of user accounts with TOTP two-factor authentication enabled.",
			nil, nil,
		),
		usersPending: prometheus.NewDesc(
			"leafwiki_users_pending_invite",
			"Current number of invited user accounts that have not yet completed sign-up.",
			nil, nil,
		),
	}
}

func (c *runtimeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pages
	ch <- c.users
	ch <- c.usersWithTOTP
	ch <- c.usersPending
}

// Collect emits the current gauge values. A source error skips only the
// affected gauges for this scrape (logged at debug, since a persistent
// failure would otherwise flood the log on every scrape interval) and keeps
// the rest of /metrics serving normally.
func (c *runtimeCollector) Collect(ch chan<- prometheus.Metric) {
	if ps, err := c.src.PageStats(); err != nil {
		c.log.Debug("skipping page gauges for this scrape", "error", err)
	} else {
		ch <- prometheus.MustNewConstMetric(c.pages, prometheus.GaugeValue, float64(ps.Pages), "page")
		ch <- prometheus.MustNewConstMetric(c.pages, prometheus.GaugeValue, float64(ps.Sections), "section")
	}

	us, err := c.src.UserStats()
	if err != nil {
		c.log.Debug("skipping user gauges for this scrape", "error", err)
		return
	}

	roles := make([]string, 0, len(us.ByRole))
	for role := range us.ByRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		ch <- prometheus.MustNewConstMetric(c.users, prometheus.GaugeValue, float64(us.ByRole[role]), role)
	}

	ch <- prometheus.MustNewConstMetric(c.usersWithTOTP, prometheus.GaugeValue, float64(us.TOTPEnabled))
	ch <- prometheus.MustNewConstMetric(c.usersPending, prometheus.GaugeValue, float64(us.PendingInvite))
}

// RegisterRuntimeStats wires a RuntimeStatsSource into the Prometheus registry
// as a scrape-time collector. It is a no-op when metrics are disabled (m nil)
// or no source is given. Call it exactly once, after the wiki is fully built.
func (m *HTTPMetrics) RegisterRuntimeStats(src RuntimeStatsSource) {
	if m == nil || src == nil {
		return
	}
	m.registry.MustRegister(newRuntimeCollector(src))
}
