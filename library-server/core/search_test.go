package main

import "testing"

func TestTypoPhraseQueriesAdjacentSwap(t *testing.T) {
	got := typoPhraseQueries("micheal jackson")
	for _, q := range got {
		if q == "michael jackson" {
			return
		}
	}
	t.Fatalf("expected michael jackson in variants, got %#v", got)
}

func TestScoreHitTypoExactTitle(t *testing.T) {
	got := scoreHit("micheal jackson", "Michael Jackson", "Michael_Jackson", "")
	if got <= 1000 {
		t.Fatalf("expected a strong score for typo against exact title, got %d", got)
	}
}
