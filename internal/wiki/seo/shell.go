package seo

import (
	"log/slog"
	"net/http"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
)

// PublicOnly gates this package's shell registrations on public access. The
// setting is read per request, so toggling it at runtime takes effect
// immediately.
type PublicOnly struct {
	gate publicaccess.ReadGate
}

// NewPublicOnly returns the gate; a nil ReadGate counts as public access off.
func NewPublicOnly(gate publicaccess.ReadGate) PublicOnly {
	return PublicOnly{gate: gate}
}

// Responder wraps r so it runs only while public access is on. Otherwise the
// request is declined before r runs, so nothing is read.
func (p PublicOnly) Responder(r httpinternal.ShellResponder) httpinternal.ShellResponder {
	return httpinternal.ShellResponderFunc(func(w http.ResponseWriter, req httpinternal.ShellRequest) bool {
		return p.enabled() && r.RespondShell(w, req)
	})
}

func (p PublicOnly) enabled() bool {
	return p.gate != nil && p.gate.Enabled()
}

// rootRedirect answers the wiki root with a 302 to the first page, so search
// engines find content at "/" instead of an empty SPA shell.
type rootRedirect struct {
	target *GetRootTargetUseCase
	log    *slog.Logger
}

// NewRootRedirect returns the shell responder for the wiki root.
func NewRootRedirect(target *GetRootTargetUseCase, log *slog.Logger) httpinternal.ShellResponder {
	return &rootRedirect{target: target, log: log}
}

func (r *rootRedirect) RespondShell(w http.ResponseWriter, req httpinternal.ShellRequest) bool {
	if req.RoutePath != "/" {
		return false
	}
	out, err := r.target.Execute(req.Request.Context(), GetRootTargetInput{BasePath: req.BasePath})
	if err != nil {
		// Swallowed here: the visitor gets the shell, whose SPA redirects itself.
		r.log.Warn("root redirect skipped, serving the shell", "error", err)
		return false
	}
	if out.Location == "" {
		return false
	}
	w.Header().Set("Location", out.Location)
	w.WriteHeader(http.StatusFound)
	return true
}
