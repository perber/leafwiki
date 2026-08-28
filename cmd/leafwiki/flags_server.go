package main

import "github.com/urfave/cli/v3"

// serverOptions holds the options for listener, data directory and logging.
type serverOptions struct {
	host              string
	port              string
	unixSocket        string
	dataDir           string
	basePath          string
	allowInsecure     bool
	logFormat         string
	disableRequestLog bool
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *serverOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "host",
			Destination: &o.host,
			Category:    catServer,
			Usage:       "host/IP address to bind the server to (e.g. 127.0.0.1 or 0.0.0.0)",
			Value:       "127.0.0.1",
			Sources:     envVars("LEAFWIKI_HOST"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "port",
			Destination: &o.port,
			Category:    catServer,
			Usage:       "port to run the server on",
			Value:       "8080",
			Sources:     envVars("LEAFWIKI_PORT"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "unix-socket",
			Destination: &o.unixSocket,
			Category:    catServer,
			Usage:       "path to a unix domain socket to listen on; overrides --host and --port",
			Sources:     envVars("LEAFWIKI_UNIX_SOCKET"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "data-dir",
			Destination: &o.dataDir,
			Category:    catServer,
			Usage:       "path to data directory",
			Value:       "./data",
			Sources:     envVars("LEAFWIKI_DATA_DIR"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "base-path",
			Destination: &o.basePath,
			Category:    catServer,
			Usage:       "URL prefix when served behind a reverse proxy (e.g. /wiki)",
			Sources:     envVars("LEAFWIKI_BASE_PATH"),
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "allow-insecure",
			Destination: &o.allowInsecure,
			Category:    catServer,
			Usage:       "allow insecure HTTP; auth cookies may then travel in plain text",
			Sources:     envBoolVars("LEAFWIKI_ALLOW_INSECURE"),
		},
		&cli.StringFlag{
			Name:             "log-format",
			Destination:      &o.logFormat,
			Category:         catLogging,
			Usage:            "log output format: text or json",
			Value:            "text",
			Sources:          envVars("LEAFWIKI_LOG_FORMAT"),
			Validator:        validateLogFormatValue,
			ValidateDefaults: true,
			Config:           trimmed,
		},
		&cli.BoolFlag{
			Name:        "disable-request-log",
			Destination: &o.disableRequestLog,
			Category:    catLogging,
			Usage:       "suppress per-request HTTP access log lines",
			Sources:     envBoolVars("LEAFWIKI_DISABLE_REQUEST_LOG"),
		},
	}
}
