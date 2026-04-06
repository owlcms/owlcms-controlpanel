package shared

import (
	"os"
	"testing"
)

func withLocaleEnvGOOS(goos string, fn func()) {
	original := localeEnvGOOS
	localeEnvGOOS = func() string { return goos }
	defer func() {
		localeEnvGOOS = original
	}()
	fn()
}

func TestNormalizeLocaleEnvironmentReplacesLangC(t *testing.T) {
	withLocaleEnvGOOS("linux", func() {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "C")
		t.Setenv("LANGUAGE", "")

		NormalizeLocaleEnvironment()

		if got := os.Getenv("LANG"); got != "en_US.UTF-8" {
			t.Fatalf("expected LANG to be normalized, got %q", got)
		}
	})
}

func TestNormalizeLocaleEnvironmentUsesLanguageFallback(t *testing.T) {
	withLocaleEnvGOOS("linux", func() {
		t.Setenv("LC_ALL", "C")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "C")
		t.Setenv("LANGUAGE", "fr_CA:fr")

		NormalizeLocaleEnvironment()

		if _, ok := os.LookupEnv("LC_ALL"); ok {
			t.Fatal("expected LC_ALL to be unset")
		}
		if got := os.Getenv("LANG"); got != "fr_CA" {
			t.Fatalf("expected LANG to use LANGUAGE fallback, got %q", got)
		}
	})
}

func TestNormalizeLocaleEnvironmentLeavesValidLocale(t *testing.T) {
	withLocaleEnvGOOS("linux", func() {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "de_DE.UTF-8")
		t.Setenv("LANGUAGE", "")

		NormalizeLocaleEnvironment()

		if got := os.Getenv("LANG"); got != "de_DE.UTF-8" {
			t.Fatalf("expected LANG to remain unchanged, got %q", got)
		}
	})
}

func TestNormalizeLocaleEnvironmentLeavesHigherPriorityLocale(t *testing.T) {
	withLocaleEnvGOOS("linux", func() {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "de_DE.UTF-8")
		t.Setenv("LANG", "C")
		t.Setenv("LANGUAGE", "")

		NormalizeLocaleEnvironment()

		if got := os.Getenv("LC_MESSAGES"); got != "de_DE.UTF-8" {
			t.Fatalf("expected LC_MESSAGES to remain unchanged, got %q", got)
		}
		if got := os.Getenv("LANG"); got != "C" {
			t.Fatalf("expected LANG to remain unchanged when a higher-priority locale exists, got %q", got)
		}
	})
}

func TestNormalizeLocaleEnvironmentNoopOnWindows(t *testing.T) {
	withLocaleEnvGOOS("windows", func() {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "C")
		t.Setenv("LANGUAGE", "")

		NormalizeLocaleEnvironment()

		if got := os.Getenv("LANG"); got != "C" {
			t.Fatalf("expected LANG to remain unchanged on non-POSIX platforms, got %q", got)
		}
	})
}
