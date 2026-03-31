package shared

import (
	"testing"
)

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name      string
		metadata  string
		wantError bool
	}{
		// Valid cases
		{"empty", "", false},
		{"ascii alphanumeric", "test123", false},
		{"ascii with hyphens", "test-123", false},
		{"ascii with dots", "test.123", false},
		{"japanese", "日本語版", false},
		{"russian", "Русский", false},
		{"mixed unicode", "test-日本語.123", false},
		{"timestamp", "2026-01-29T150405", false},
		{"emoji", "🚀test", false},

		// Invalid cases - forbidden filesystem chars
		{"less than", "test<file", true},
		{"greater than", "test>file", true},
		{"colon", "test:file", true},
		{"double quote", "test\"file", true},
		{"forward slash", "test/file", true},
		{"backslash", "test\\file", true},
		{"pipe", "test|file", true},
		{"question mark", "test?file", true},
		{"asterisk", "test*file", true}, {"plus sign", "test+file", true}, {"control char", "test\x00file", true},
		{"newline", "test\nfile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadata(tt.metadata)
			expectedResult := "legal"
			if tt.wantError {
				expectedResult = "illegal"
			}
			actualResult := "legal"
			if err != nil {
				actualResult = "illegal"
			}
			t.Logf("ValidateMetadata(%q) expected: %s, got: %s", tt.metadata, expectedResult, actualResult)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateMetadata(%q) [test: %s] expected: %s, got: %s (error: %v)", tt.metadata, tt.name, expectedResult, actualResult, err)
			}
		})
	}
}

func TestValidateVersionName(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantError bool
	}{
		{"simple version", "5.0.0", false},
		{"version with ascii metadata", "5.0.0+build123", false},
		{"version with unicode metadata", "5.0.0+日本語版", false},
		{"version with timestamp", "5.0.0+2026-01-29T150405", false},
		{"prerelease with metadata", "5.0.0-beta.1+test", false},
		{"complex unicode", "5.0.0+日本語-test.123", false},

		{"empty version", "", true},
		{"metadata with forbidden char", "5.0.0+test<bad", true},
		{"metadata with slash", "5.0.0+test/bad", true}, {"multiple plus signs", "5.0.0+test+extra", true},
		{"plus in metadata", "5.0.0+build+123", true}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersionName(tt.version)
			expectedResult := "legal"
			if tt.wantError {
				expectedResult = "illegal"
			}
			actualResult := "legal"
			if err != nil {
				actualResult = "illegal"
			}
			t.Logf("ValidateVersionName(%q) expected: %s, got: %s", tt.version, expectedResult, actualResult)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateVersionName(%q) [test: %s] expected: %s, got: %s (error: %v)", tt.version, tt.name, expectedResult, actualResult, err)
			}
		})
	}
}

func TestSanitizeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{"clean ascii", "test123", "test123", false},
		{"clean unicode", "日本語版", "日本語版", false},
		{"remove forbidden", "test<bad>file", "testbadfile", true},
		{"remove multiple", "a:b/c\\d", "abcd", true},
		{"remove control char", "test\x00bad", "testbad", true}, {"replace plus with dot", "test+123", "test.123", true},
		{"replace multiple plus", "a+b+c", "a.b.c", true}, {"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := SanitizeMetadata(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeMetadata(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			if changed != tt.changed {
				t.Errorf("SanitizeMetadata(%q) changed = %v, want %v", tt.input, changed, tt.changed)
			}
		})
	}
}

func TestNormalizeVersionName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no metadata", "5.0.0", "5.0.0"},
		{"clean metadata", "5.0.0+build123", "5.0.0+build123"},
		{"plus in metadata", "5.0.0+build+123", "5.0.0+build.123"},
		{"multiple plus", "5.0.0+a+b+c", "5.0.0+a.b.c"},
		{"forbidden chars", "5.0.0+test<bad>", "5.0.0+testbad"},
		{"mixed issues", "5.0.0+a+b/c", "5.0.0+a.bc"},
		{"all forbidden removed", "5.0.0+<>:", "5.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeVersionName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersionName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripMetadata(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"no metadata", "5.0.0", "5.0.0"},
		{"with metadata", "5.0.0+build123", "5.0.0"},
		{"with unicode metadata", "5.0.0+日本語版", "5.0.0"},
		{"prerelease with metadata", "5.0.0-beta.1+test", "5.0.0-beta.1"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripMetadata(tt.version)
			if result != tt.expected {
				t.Errorf("StripMetadata(%q) = %q, want %q", tt.version, result, tt.expected)
			}
		})
	}
}

func TestSanitizeVersionBuild(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ascii", "test123", "test123"},
		{"spaces become dots", "my version", "my.version"},
		{"accented preserved", "équipes", "équipes"},
		{"japanese preserved", "日本語", "日本語"},
		{"forbidden chars removed", "test<bad>", "testbad"},
		{"plus removed", "a+b", "ab"},
		{"leading trailing stripped", ".-test-.", "test"},
		{"consecutive dots collapsed", "a..b", "a.b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeVersionBuild(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeVersionBuild(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractVersionFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"plain version", "owlcms_5.0.0.zip", "5.0.0", false},
		{"ascii metadata", "owlcms_5.0.0+build123.zip", "5.0.0+build123", false},
		{"accented metadata preserved", "owlcms_5.0.0+équipes.zip", "5.0.0+équipes", false},
		{"japanese metadata preserved", "owlcms_5.0.0+日本語版.zip", "5.0.0+日本語版", false},
		{"accented base normalized", "owlcms_5.0.0+TEST.zip", "5.0.0+TEST", false},
		{"no zip extension", "owlcms_5.0.0", "", true},
		{"invalid semver", "owlcms_notaversion.zip", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractVersionFromFilename(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractVersionFromFilename(%q) expected error, got %q", tt.input, result)
				}
				return
			}
			if err != nil {
				t.Errorf("ExtractVersionFromFilename(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ExtractVersionFromFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasUnicodeLetters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ascii only", "test123", false},
		{"japanese", "日本語", true},
		{"russian", "Русский", true},
		{"mixed", "test日本語", true},
		{"emoji not letter", "test🚀", false}, // emoji are not letters
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasUnicodeLetters(tt.input)
			if result != tt.expected {
				t.Errorf("HasUnicodeLetters(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
