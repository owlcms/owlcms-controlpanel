package owlcms

import "testing"

func TestShouldUseOwlcmsDaemonWrapperForDetachedDaemonMode(t *testing.T) {
	t.Setenv("CONTROLPANEL_RUN_AS_DAEMON", "true")
	t.Setenv("INVOCATION_ID", "")

	if !shouldUseOwlcmsDaemonWrapper() {
		t.Fatalf("expected detached daemon launches to use MainWrapper")
	}
}

func TestShouldUseOwlcmsDaemonWrapperDisabledUnderSystemd(t *testing.T) {
	t.Setenv("CONTROLPANEL_RUN_AS_DAEMON", "true")
	t.Setenv("INVOCATION_ID", "systemd-123")

	if shouldUseOwlcmsDaemonWrapper() {
		t.Fatalf("expected systemd launches to skip MainWrapper")
	}
}

func TestBuildOwlcmsCommandUsesMainWrapperForDaemonMode(t *testing.T) {
	params := &owlcmsLaunchParams{
		JavaPath: "/tmp/java",
		JarPath:  "/tmp/owlcms.jar",
	}

	cmd := buildOwlcmsCommand(params, true)

	want := []string{"/tmp/java", "-cp", "/tmp/owlcms.jar", "app.owlcms.MainWrapper"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("unexpected args length: got %v want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("unexpected arg %d: got %q want %q (all args: %v)", i, cmd.Args[i], want[i], cmd.Args)
		}
	}
}

func TestBuildOwlcmsCommandUsesJarEntrypointForNormalMode(t *testing.T) {
	params := &owlcmsLaunchParams{
		JavaPath: "/tmp/java",
		JarPath:  "/tmp/owlcms.jar",
	}

	cmd := buildOwlcmsCommand(params, false)

	want := []string{"/tmp/java", "-jar", "owlcms.jar"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("unexpected args length: got %v want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("unexpected arg %d: got %q want %q (all args: %v)", i, cmd.Args[i], want[i], want)
		}
	}
}

func TestSetEnvValueAppendsWhenMissing(t *testing.T) {
	env := []string{"A=1", "B=2"}
	env = setEnvValue(env, embeddedMQTTEnv, "false")

	if got := env[len(env)-1]; got != embeddedMQTTEnv+"=false" {
		t.Fatalf("expected appended mqtt env, got %q", got)
	}
}

func TestSetEnvValueReplacesExistingValue(t *testing.T) {
	env := []string{"A=1", embeddedMQTTEnv + "=true", "B=2"}
	env = setEnvValue(env, embeddedMQTTEnv, "false")

	var matches int
	for _, entry := range env {
		if entry == embeddedMQTTEnv+"=false" {
			matches++
		}
		if entry == embeddedMQTTEnv+"=true" {
			t.Fatalf("expected existing mqtt env to be replaced, env=%v", env)
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one mqtt env entry, got %d in %v", matches, env)
	}
}
