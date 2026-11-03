package diff

import "testing"

func TestSummary(t *testing.T) {
	tests := map[string]string{
		"":                                     "",
		"Manages a server. More text here.":    "Manages a server.",
		"Use e.g. foo. Then bar.":              "Use e.g. foo.",
		"Version 1.2 is required. Other.":      "Version 1.2 is required.",
		"First paragraph\nwrapped.\n\nSecond.": "First paragraph wrapped.",
		"No final period":                      "No final period",
	}
	for in, want := range tests {
		if got := summary(in); got != want {
			t.Errorf("summary(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterNames(t *testing.T) {
	names := []string{"registry.terraform.io/hashicorp/aws", "registry.terraform.io/hashicorp/awscc"}
	got := filterNames(names, "aws")
	if len(got) != 1 || got[0] != names[0] {
		t.Errorf("filterNames(aws) = %v", got)
	}
	if got := filterNames(names, "hashicorp/awscc"); len(got) != 1 || got[0] != names[1] {
		t.Errorf("filterNames(hashicorp/awscc) = %v", got)
	}
}

func TestDeprecationNote(t *testing.T) {
	tests := map[string]string{
		"Include numeric characters. Default is `true`. **NOTE**: This is deprecated, use `numeric` instead.": "Use `numeric` instead.",
		"Manages a volume. Use demo_disk instead.":                                                            "Use demo_disk instead.",
		"~> **Deprecated**: replaced by `foo`.":                                                               "Replaced by `foo`.",
		"This attribute is deprecated and will be removed in v4, use `b`.":                                    "Use `b`.",
		"Just a description. Nothing else.":                                                                   "Just a description.",
		"":                                                                                                    "",
	}
	for in, want := range tests {
		if got := deprecationNote(in); got != want {
			t.Errorf("deprecationNote(%q) = %q, want %q", in, got, want)
		}
	}
}
