package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/perber/wiki/internal/core/auth"
)

const (
	catServer    = "Server"
	catLogging   = "Logging"
	catAuth      = "Authentication"
	catAdmin     = "Admin bootstrap"
	catProxyAuth = "Reverse-proxy authentication"
	catURLs      = "External URLs"
	catFrontend  = "Frontend"
	catFeatures  = "Features"
	catRevisions = "Revisions"
	catMetrics   = "Metrics"
	catGitBackup = "Git backup"
	catSnapshots = "Snapshots & restore"
	catEmail     = "Email (SMTP)"
)

const rootUsageText = `leafwiki --jwt-secret <SECRET> --admin-password <PASSWORD> [--host <HOST>] [--port <PORT>] [--unix-socket <PATH>] [--data-dir <DIR>]
leafwiki --disable-auth [--host <HOST>] [--port <PORT>] [--unix-socket <PATH>] [--data-dir <DIR>]
leafwiki reset-admin-password
leafwiki [--data-dir <DIR>] restore-snapshot <path-to-zip>
leafwiki --help`

const rootDescription = `Every option can also be set through the environment variable shown next to it.
Command line flags win over environment variables, which win over the defaults.
An environment variable that is set but empty counts as unset.

LEAFWIKI_LOG_LEVEL (debug, info, warn, error; default info) is available as an
environment variable only.

When --snapshot is enabled, live restore-from-snapshot is also available via the
admin UI (Settings > Full Backup) and gates writes (503) while a restore is
swapping files. For disaster recovery or migrating a snapshot to a fresh
instance, use the "restore-snapshot" command instead: run it before starting the
server against that data directory.`

// serverConfig is every resolved option, grouped the way --help groups them.
// Each group owns its declarations in its own file, so adding an option means
// touching one struct and one literal next to each other.
type serverConfig struct {
	server   serverOptions
	auth     authOptions
	proxy    proxyOptions
	frontend frontendOptions
	metrics  metricsOptions
	backup   backupOptions
	email    emailOptions
}

// flags collects the groups. A group missing from this list would silently
// have no flags, which TestRootCommandHelp_DocumentsFlagsAndEnvVars catches.
func (c *serverConfig) flags() []cli.Flag {
	return slices.Concat(
		c.server.Flags(),
		c.auth.Flags(),
		c.proxy.Flags(),
		c.frontend.Flags(),
		c.metrics.Flags(),
		c.backup.Flags(),
		c.email.Flags(),
	)
}

// newRootCommand builds the leafwiki command tree. Running it without a
// subcommand starts the server.
func newRootCommand() *cli.Command {
	return newRootCommandWithConfig(&serverConfig{})
}

func newRootCommandWithConfig(cfg *serverConfig) *cli.Command {
	return &cli.Command{
		Name:        "LeafWiki",
		Usage:       "lightweight selfhosted wiki 🌿",
		UsageText:   rootUsageText,
		Description: rootDescription,
		Version:     Version,
		Suggest:     true,
		// Adds a "completion" command emitting bash, zsh, fish and PowerShell
		// scripts, generated from the same declarations as --help.
		EnableShellCompletion: true,
		// urfave/cli would print the error itself and dump the whole help; it
		// reports the same failure main would, so handle it here once and hand
		// back errReported to keep main quiet.
		OnUsageError: reportUsageError,
		Flags:        cfg.flags(),
		Commands: []*cli.Command{
			newResetAdminPasswordCommand(cfg),
			newRestoreSnapshotCommand(cfg),
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// The log-format validator has already rejected anything parseLogFormat
			// cannot handle, including the default; this only normalizes the case.
			format, _ := parseLogFormat(cfg.server.logFormat)
			setupLogger(os.Stdout, format)
			return ctx, nil
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Present() {
				return cli.Exit(fmt.Sprintf("Unknown command: %s\nRun \"leafwiki --help\" to see the available commands and options.", cmd.Args().First()), 1)
			}
			return runServerCommand(ctx, cmd, cfg)
		},
	}
}

func newResetAdminPasswordCommand(cfg *serverConfig) *cli.Command {
	return &cli.Command{
		Name:  "reset-admin-password",
		Usage: "Reset the admin password and print the new one",
		Action: func(_ context.Context, _ *cli.Command) error {
			return runResetAdminPasswordCommand(cfg)
		},
	}
}

func newRestoreSnapshotCommand(cfg *serverConfig) *cli.Command {
	return &cli.Command{
		Name:      "restore-snapshot",
		Usage:     "Restore a snapshot ZIP into the data directory while the server is stopped",
		ArgsUsage: "<path-to-zip>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runRestoreSnapshotCommand(cfg.server.dataDir, cmd.Args().First())
		},
	}
}

// reportUsageError prints a flag parsing failure, with urfave/cli's "did you
// mean" hint when it can name the offending flag, and points at --help instead
// of dumping every option.
func reportUsageError(_ context.Context, cmd *cli.Command, err error, _ bool) error {
	out := cmd.Root().ErrWriter
	_, _ = fmt.Fprintf(out, "Incorrect usage: %s\n", err)
	if name := unknownFlagName(err); name != "" && cmd.Root().Suggest {
		if suggestion := cli.SuggestFlag(cmd.Flags, name, true); suggestion != "" && suggestion != name {
			_, _ = fmt.Fprintf(out, "Did you mean %q?\n", suggestion)
		}
	}
	_, _ = fmt.Fprintf(out, "Run %q to see the available commands and options.\n", "leafwiki --help")
	return errReported
}

// unknownFlagName digs the rejected flag out of a parse error so a suggestion
// can be offered. The error text belongs to the flag package, so a miss just
// means no hint.
func unknownFlagName(err error) string {
	_, name, found := strings.Cut(err.Error(), "not defined: ")
	if !found {
		return ""
	}
	return strings.TrimLeft(name, "-")
}

// Flag validators run for values from the command line and from the
// environment alike, and - with ValidateDefaults set - for the declared default
// too. A flag Action would not: urfave/cli only runs those for flags that were
// actually set, so a default would go unchecked.

func validateLogFormatValue(v string) error {
	if _, ok := parseLogFormat(v); !ok {
		return fmt.Errorf("expected text or json, got %q", v)
	}
	return nil
}

func validateByteSizeValue(v string) error {
	_, err := parseByteSize(v)
	return err
}

func redirectURLValidator(flagName string) func(string) error {
	return func(v string) error {
		return validateRedirectURL(flagName, v)
	}
}

func validateTOTPEncryptionKey(v string) error {
	if v != "" && len(v) < auth.MinTOTPEncryptionKeyLen {
		return fmt.Errorf("key is too short: need at least %d bytes, got %d", auth.MinTOTPEncryptionKeyLen, len(v))
	}
	return nil
}

// trimmed makes a string flag drop surrounding whitespace, for both command
// line and environment variable values.
var trimmed = cli.StringConfig{TrimSpace: true}

// envVars is cli.EnvVars with LeafWiki's long-standing environment variable
// semantics: a variable that is set but empty (or only whitespace) counts as
// unset, and values are whitespace trimmed. Deployments rely on this, since
// compose and .env files routinely declare variables without a value - taking
// an empty LEAFWIKI_HOST literally would bind the server to every interface.
func envVars(keys ...string) cli.ValueSourceChain {
	sources := make([]cli.ValueSource, 0, len(keys))
	for _, key := range keys {
		sources = append(sources, &envVarSource{key: key})
	}
	return cli.NewValueSourceChain(sources...)
}

// envBoolVars is envVars for bool flags. It additionally accepts the spellings
// LeafWiki has always allowed in environment variables (yes/no, on/off, y/n),
// which strconv.ParseBool - and therefore the command line - does not. An
// unparseable value is handed to the flag as-is so that urfave/cli reports it
// and startup fails fast.
func envBoolVars(keys ...string) cli.ValueSourceChain {
	sources := make([]cli.ValueSource, 0, len(keys))
	for _, key := range keys {
		sources = append(sources, &envVarSource{key: key, normalize: normalizeBoolValue})
	}
	return cli.NewValueSourceChain(sources...)
}

func normalizeBoolValue(raw string) string {
	if b, ok := parseBool(raw); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return raw
}

// envVarSource is a cli.ValueSource over one environment variable. It also
// implements cli.EnvValueSource so that generated help still shows the
// variable name next to its flag.
type envVarSource struct {
	key       string
	normalize func(string) string
}

func (e *envVarSource) Lookup() (string, bool) {
	val := strings.TrimSpace(os.Getenv(e.key))
	if val == "" {
		return "", false
	}
	if e.normalize != nil {
		val = e.normalize(val)
	}
	return val, true
}

func (e *envVarSource) IsFromEnv() bool { return true }

func (e *envVarSource) Key() string { return e.key }

func (e *envVarSource) String() string { return fmt.Sprintf("environment variable %q", e.key) }

func (e *envVarSource) GoString() string { return fmt.Sprintf("&envVarSource{Key:%q}", e.key) }
