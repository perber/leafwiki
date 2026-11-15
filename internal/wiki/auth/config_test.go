package auth

import (
	"testing"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/pagedisplay"
	"github.com/perber/wiki/internal/publicaccess"
)

// The page display settings are merged into /api/config flat, after the
// built-in fields; a display setting named like a built-in would silently
// replace it. This pins that no name is shared.
func TestBaseConfig_PageDisplayFields_DoNotShadowBuiltInKeys(t *testing.T) {
	base := baseConfig(httpinternal.RouterOptions{PublicAccess: publicaccess.Fixed(false)})
	if len(base) == 0 {
		t.Fatal("baseConfig returned no fields")
	}
	for key := range (pagedisplay.Settings{}).ConfigFields() {
		if _, ok := base[key]; ok {
			t.Errorf("page display field %q collides with a built-in /api/config key", key)
		}
	}
}
