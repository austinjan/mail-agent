package main

import "testing"

func TestVersionStringNotEmpty(t *testing.T) {
	if versionString() == "" {
		t.Fatal("versionString should never return empty")
	}
}
