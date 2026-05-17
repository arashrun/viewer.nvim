package main

import "testing"

func TestResolveDashKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filetype string
		want     string
	}{
		{name: "cpp", filetype: "cpp", want: "cpp"},
		{name: "c", filetype: "c", want: "c"},
		{name: "c++", filetype: "c++", want: "c++"},
		{name: "cc", filetype: "cc", want: "cpp"},
		{name: "typescriptreact", filetype: "typescriptreact", want: "javascript"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveDashKeys(tt.filetype); got != tt.want {
				t.Fatalf("resolveDashKeys(%q) = %q, want %q", tt.filetype, got, tt.want)
			}
		})
	}
}

func TestNormalizeDashQuery(t *testing.T) {
	t.Parallel()

	if got := normalizeDashQuery("cpp", "vector"); got != "cpp:vector" {
		t.Fatalf("normalizeDashQuery(cpp) = %q, want %q", got, "cpp:vector")
	}
	if got := normalizeDashQuery("c", "vector"); got != "c:vector" {
		t.Fatalf("normalizeDashQuery(c) = %q, want %q", got, "c:vector")
	}
}
