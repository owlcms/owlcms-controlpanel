package owlcms

import "testing"

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
