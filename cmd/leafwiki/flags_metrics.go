package main

import "github.com/urfave/cli/v3"

// metricsOptions holds the options for the Prometheus metrics listener.
type metricsOptions struct {
	enableMetrics bool
	metricsHost   string
	metricsPort   string
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *metricsOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:        "enable-metrics",
			Destination: &o.enableMetrics,
			Category:    catMetrics,
			Usage:       "enable the Prometheus /metrics endpoint on a separate listener",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_METRICS"),
		},
		&cli.StringFlag{
			Name:        "metrics-host",
			Destination: &o.metricsHost,
			Category:    catMetrics,
			Usage:       "host/IP address for the Prometheus metrics listener",
			Value:       "127.0.0.1",
			Sources:     envVars("LEAFWIKI_METRICS_HOST"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "metrics-port",
			Destination: &o.metricsPort,
			Category:    catMetrics,
			Usage:       "port for the Prometheus metrics listener",
			Value:       "9091",
			Sources:     envVars("LEAFWIKI_METRICS_PORT"),
			Config:      trimmed,
		},
	}
}
