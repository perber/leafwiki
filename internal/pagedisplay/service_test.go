package pagedisplay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func newTestService(t *testing.T, dir string) *Service {
	t.Helper()
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New(%s): %v", dir, err)
	}
	return svc
}

func TestService_NoFile_DisabledByDefault(t *testing.T) {
	svc := newTestService(t, t.TempDir())
	if svc.Settings().AlwaysShowToc {
		t.Fatal("expected AlwaysShowToc false when toc-display.json is absent")
	}
}

func TestService_SetAlwaysShow_PersistsAcrossReconstruction(t *testing.T) {
	dir := t.TempDir()

	svc := newTestService(t, dir)
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}
	if !svc.Settings().AlwaysShowToc {
		t.Fatal("expected AlwaysShowToc true right after SetAlwaysShow(true)")
	}

	reopened := newTestService(t, dir)
	if !reopened.Settings().AlwaysShowToc {
		t.Fatal("expected persisted AlwaysShowToc true after reconstruction")
	}

	if err := reopened.SetAlwaysShow(false); err != nil {
		t.Fatalf("SetAlwaysShow(false): %v", err)
	}
	again := newTestService(t, dir)
	if again.Settings().AlwaysShowToc {
		t.Fatal("expected persisted AlwaysShowToc false after SetAlwaysShow(false)")
	}
}

func TestService_SetAlwaysShow_WritesModeSixHundredJSON(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t, dir)
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}

	path := filepath.Join(dir, "toc-display.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat toc-display.json: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected toc-display.json mode 0600, got %o", perm)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read toc-display.json: %v", err)
	}
	var cfg struct {
		AlwaysShow bool `json:"alwaysShow"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal toc-display.json: %v", err)
	}
	if !cfg.AlwaysShow {
		t.Fatalf("expected {\"alwaysShow\": true} on disk, got %s", data)
	}
}

func TestService_Reload_PicksUpExternalFileChange(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t, dir)
	if svc.Settings().AlwaysShowToc {
		t.Fatal("precondition: expected disabled")
	}

	// Simulate a restore dropping in a different toc-display.json.
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte(`{"alwaysShow": true}`), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	if svc.Settings().AlwaysShowToc {
		t.Fatal("expected cache to still read false before Reload()")
	}

	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !svc.Settings().AlwaysShowToc {
		t.Fatal("expected AlwaysShowToc true after Reload() picked up the new file")
	}
}

// A data directory written before the tocdisplay -> pagedisplay rename
// holds toc-display.json; the renamed package must keep reading it.
func TestService_New_ReadsExistingTocDisplayFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte(`{"alwaysShow":true}`), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	svc := newTestService(t, dir)
	if !svc.Settings().AlwaysShowToc {
		t.Fatal("expected AlwaysShowToc true from a pre-existing toc-display.json")
	}
}

func TestService_Load_CorruptFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("expected New to fail on an unparseable toc-display.json")
	}
}

func TestService_ConcurrentAccess_IsRaceFree(t *testing.T) {
	svc := newTestService(t, t.TempDir())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(v int) { defer wg.Done(); _ = svc.SetAlwaysShow(v%2 == 0) }(i)
		go func() { defer wg.Done(); _ = svc.Settings() }()
		wg.Add(1)
		go func() { defer wg.Done(); _ = svc.SetContentWidth(ContentWidthWide) }()
		// Reload shares the setters' lock; this also guards against a
		// lock-order deadlock between it and the store's own lock.
		wg.Add(1)
		go func() { defer wg.Done(); _ = svc.Reload() }()
	}
	wg.Wait()
}

func TestSettings_ConfigFields_UsesTheConfigAPINames(t *testing.T) {
	got := Settings{AlwaysShowToc: true, ContentWidth: ContentWidthWide}.ConfigFields()
	if got["alwaysShowToc"] != true {
		t.Fatalf("ConfigFields()[alwaysShowToc] = %v, want true (all fields: %v)", got["alwaysShowToc"], got)
	}
	if got["contentWidth"] != ContentWidthWide {
		t.Fatalf("ConfigFields()[contentWidth] = %v, want %q (all fields: %v)", got["contentWidth"], ContentWidthWide, got)
	}
}

func TestFixed_Settings_ReturnsPinnedSettings(t *testing.T) {
	want := Settings{AlwaysShowToc: true}
	if got := Fixed(want).Settings(); got != want {
		t.Fatalf("Fixed(%+v).Settings() = %+v", want, got)
	}
}

// FileName is wired into snapshots and restore by name; it must keep
// pointing at the file existing data directories already hold.
func TestFileName_IsTheHistoricalTocDisplayFile(t *testing.T) {
	if FileName != "toc-display.json" {
		t.Fatalf("FileName = %q, want %q", FileName, "toc-display.json")
	}
}

func TestService_ContentWidth_ReadsFileValueOrFallsBackToDefault(t *testing.T) {
	tests := []struct {
		name string
		file string // "" = no file
		want string
	}{
		{name: "no file", file: "", want: ContentWidthDefault},
		{name: "file written before content width existed", file: `{"alwaysShow":true}`, want: ContentWidthDefault},
		{name: "empty value", file: `{"contentWidth":""}`, want: ContentWidthDefault},
		{name: "unknown value from a hand-edited file", file: `{"contentWidth":"huge"}`, want: ContentWidthDefault},
		{name: "default", file: `{"contentWidth":"default"}`, want: ContentWidthDefault},
		{name: "wide", file: `{"contentWidth":"wide"}`, want: ContentWidthWide},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte(tc.file), 0o600); err != nil {
					t.Fatalf("write file: %v", err)
				}
			}
			if got := newTestService(t, dir).Settings().ContentWidth; got != tc.want {
				t.Fatalf("ContentWidth = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestService_SetContentWidth_PersistsAcrossReconstruction(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t, dir)
	if err := svc.SetContentWidth(ContentWidthWide); err != nil {
		t.Fatalf("SetContentWidth(wide): %v", err)
	}
	if got := svc.Settings().ContentWidth; got != ContentWidthWide {
		t.Fatalf("ContentWidth right after set = %q, want %q", got, ContentWidthWide)
	}
	if got := newTestService(t, dir).Settings().ContentWidth; got != ContentWidthWide {
		t.Fatalf("ContentWidth after reconstruction = %q, want %q", got, ContentWidthWide)
	}
}

func TestService_SetContentWidth_InvalidValueIsRejectedAndNotStored(t *testing.T) {
	for _, value := range []string{"", "huge", "Wide", " wide", "wide ", "WIDE", "default\n", "wide\u0000", "wíde"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			svc := newTestService(t, dir)
			err := svc.SetContentWidth(value)
			loc, ok := sharederrors.AsLocalizedError(err)
			if !ok || loc.Code != ErrCodeInvalidContentWidth {
				t.Fatalf("SetContentWidth(%q) error = %v, want code %q", value, err, ErrCodeInvalidContentWidth)
			}
			if got := svc.Settings().ContentWidth; got != ContentWidthDefault {
				t.Fatalf("ContentWidth after rejected set = %q, want %q", got, ContentWidthDefault)
			}
			if _, err := os.Stat(filepath.Join(dir, "toc-display.json")); !os.IsNotExist(err) {
				t.Fatalf("expected no file written for a rejected value, stat err = %v", err)
			}
		})
	}
}

// Both settings share one file: changing one must not reset the other.
func TestService_Setters_KeepTheOtherSetting(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t, dir)
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}
	if err := svc.SetContentWidth(ContentWidthWide); err != nil {
		t.Fatalf("SetContentWidth(wide): %v", err)
	}
	if err := svc.SetAlwaysShow(false); err != nil {
		t.Fatalf("SetAlwaysShow(false): %v", err)
	}
	want := Settings{AlwaysShowToc: false, ContentWidth: ContentWidthWide}
	if got := newTestService(t, dir).Settings(); got != want {
		t.Fatalf("Settings after reconstruction = %+v, want %+v", got, want)
	}
	if err := svc.SetContentWidth(ContentWidthDefault); err != nil {
		t.Fatalf("SetContentWidth(default): %v", err)
	}
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}
	want = Settings{AlwaysShowToc: true, ContentWidth: ContentWidthDefault}
	if got := newTestService(t, dir).Settings(); got != want {
		t.Fatalf("Settings after second round = %+v, want %+v", got, want)
	}
}

func TestService_Reload_PicksUpExternalContentWidth(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte(`{"contentWidth":"wide"}`), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got := svc.Settings().ContentWidth; got != ContentWidthWide {
		t.Fatalf("ContentWidth after Reload = %q, want %q", got, ContentWidthWide)
	}
}

func TestContentWidths_StartWithTheDefault(t *testing.T) {
	got := ContentWidths()
	if len(got) < 2 || got[0] != ContentWidthDefault {
		t.Fatalf("ContentWidths() = %v, want %q first and at least one wider option", got, ContentWidthDefault)
	}
}

// Concurrent admins changing different settings: every setter does a
// read-modify-write of the shared file, so without serialisation the last
// writer would put back a stale copy of the other setting.
func TestService_ConcurrentSetters_DoNotClobberEachOther(t *testing.T) {
	for round := 0; round < 20; round++ {
		dir := t.TempDir()
		svc := newTestService(t, dir)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(2)
			go func() { defer wg.Done(); _ = svc.SetAlwaysShow(true) }()
			go func() { defer wg.Done(); _ = svc.SetContentWidth(ContentWidthWide) }()
		}
		wg.Wait()
		want := Settings{AlwaysShowToc: true, ContentWidth: ContentWidthWide}
		if got := svc.Settings(); got != want {
			t.Fatalf("round %d: in-memory settings = %+v, want %+v", round, got, want)
		}
		if got := newTestService(t, dir).Settings(); got != want {
			t.Fatalf("round %d: settings on disk = %+v, want %+v", round, got, want)
		}
	}
}

// A file written by an older version (or edited by hand) must survive the
// other setter untouched: the read path normalises, the write path must not
// drop or rewrite fields it does not own.
func TestService_Setters_OnPreExistingFile(t *testing.T) {
	tests := []struct {
		name string
		file string
		set  func(*Service) error
		want Settings
	}{
		{
			name: "content width on a file from before the setting existed keeps alwaysShow",
			file: `{"alwaysShow":true}`,
			set:  func(s *Service) error { return s.SetContentWidth(ContentWidthWide) },
			want: Settings{AlwaysShowToc: true, ContentWidth: ContentWidthWide},
		},
		{
			name: "alwaysShow on a file with an unknown width still reads the default width",
			file: `{"alwaysShow":false,"contentWidth":"huge"}`,
			set:  func(s *Service) error { return s.SetAlwaysShow(true) },
			want: Settings{AlwaysShowToc: true, ContentWidth: ContentWidthDefault},
		},
		{
			name: "content width replaces an unknown stored width",
			file: `{"alwaysShow":true,"contentWidth":"Wide"}`,
			set:  func(s *Service) error { return s.SetContentWidth(ContentWidthWide) },
			want: Settings{AlwaysShowToc: true, ContentWidth: ContentWidthWide},
		},
		{
			name: "unknown keys from a newer version do not break loading or writing",
			file: `{"alwaysShow":true,"contentWidth":"wide","futureSetting":42}`,
			set:  func(s *Service) error { return s.SetAlwaysShow(false) },
			want: Settings{AlwaysShowToc: false, ContentWidth: ContentWidthWide},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tc.file), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			svc := newTestService(t, dir)
			if err := tc.set(svc); err != nil {
				t.Fatalf("set: %v", err)
			}
			if got := svc.Settings(); got != tc.want {
				t.Fatalf("in-memory settings = %+v, want %+v", got, tc.want)
			}
			if got := newTestService(t, dir).Settings(); got != tc.want {
				t.Fatalf("settings on disk = %+v, want %+v", got, tc.want)
			}
		})
	}
}
