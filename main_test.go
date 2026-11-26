package main

import (
	"slices"
	"testing"
)

func TestResumeArgv(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"bare", nil, []string{"jiru"}},
		{"short profile", []string{"-p", "work"}, []string{"jiru", "-p", "work"}},
		{"long profile", []string{"--profile", "work"}, []string{"jiru", "--profile", "work"}},
		{"long profile with equals", []string{"--profile=work"}, []string{"jiru", "--profile=work"}},
		{"profile flag without value", []string{"-p"}, []string{"jiru"}},
		{"short key", []string{"-k", "PROJ-123"}, []string{"jiru", "-k", "PROJ-123"}},
		{"long key", []string{"--key", "PROJ-123"}, []string{"jiru", "--key", "PROJ-123"}},
		{"long key with equals", []string{"--key=PROJ-123"}, []string{"jiru", "--key=PROJ-123"}},
		{"profile and key", []string{"-p", "work", "-k", "PROJ-123"}, []string{"jiru", "-p", "work", "-k", "PROJ-123"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resumeArgv(tt.args); !slices.Equal(got, tt.want) {
				t.Errorf("resumeArgv(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
