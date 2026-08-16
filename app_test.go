package main

import "testing"

func TestCheckUpdateSkipsDevBuilds(t *testing.T) {
	if got := (&App{}).CheckUpdate(); got != "" {
		t.Fatalf("dev build must never report an update, got %q", got)
	}
}
