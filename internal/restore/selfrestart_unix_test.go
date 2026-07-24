//go:build !windows

package restore

import (
	"os"
	"reflect"
	"testing"
)

func TestSelfRestart_CallsExecWithCurrentExecutableArgsAndEnv(t *testing.T) {
	orig := execFn
	t.Cleanup(func() { execFn = orig })

	var gotArgv0 string
	var gotArgv, gotEnv []string
	execFn = func(argv0 string, argv, envv []string) error {
		gotArgv0, gotArgv, gotEnv = argv0, argv, envv
		return nil
	}

	if err := SelfRestart(); err != nil {
		t.Fatalf("SelfRestart() error = %v", err)
	}

	wantExe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	if gotArgv0 != wantExe {
		t.Errorf("execFn called with argv0 = %q, want %q", gotArgv0, wantExe)
	}
	if !reflect.DeepEqual(gotArgv, os.Args) {
		t.Errorf("execFn called with argv = %v, want %v", gotArgv, os.Args)
	}
	if !reflect.DeepEqual(gotEnv, os.Environ()) {
		t.Errorf("execFn called with envv = %v, want %v", gotEnv, os.Environ())
	}
}

func TestSelfRestart_PropagatesExecError(t *testing.T) {
	orig := execFn
	t.Cleanup(func() { execFn = orig })

	wantErr := os.ErrPermission
	execFn = func(argv0 string, argv, envv []string) error { return wantErr }

	if err := SelfRestart(); err != wantErr {
		t.Errorf("SelfRestart() error = %v, want %v", err, wantErr)
	}
}
