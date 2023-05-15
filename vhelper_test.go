package main

import (
	"testing"
)

func TestAfter1(t *testing.T) {
	s := "23:34:45.234"
	res := after(s, ":")
	expected := "45.234"

	if res != expected {
		t.Errorf("Result: %v, want: %v", res, expected)
	}
}

func TestAfter2(t *testing.T) {
	s := "abcd"
	res := after(s, "z")
	expected := s

	if res != expected {
		t.Errorf("Result: %v, want: %v", res, expected)
	}
}
