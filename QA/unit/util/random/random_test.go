package random_test

import (
	"log"
	"testing"

	"x-ui/util/random"
)

func TestSeq(t *testing.T) {
	n := 10
	seq := random.Seq(n)
	if len(seq) != n {
		t.Errorf("Seq(%d) length = %d, want %d", n, len(seq), n)
	}

	n = 20
	seq2 := random.Seq(n)
	if len(seq2) != n {
		t.Errorf("Seq(%d) length = %d, want %d", n, len(seq2), n)
	}

	if seq == seq2 {
		t.Log("Note: Seq generated same sequence twice (very unlikely but possible)")
	}

	log.Printf("Random Seq(10): %s", seq)
}

func TestNum(t *testing.T) {
	n := 10
	val := random.Num(n)
	if val < 0 || val >= n {
		t.Errorf("Num(%d) = %d, want [0, %d)", n, val, n)
	}

	n = 100
	for i := 0; i < 100; i++ {
		val = random.Num(n)
		if val < 0 || val >= n {
			t.Errorf("Num(%d) = %d, want [0, %d)", n, val, n)
		}
	}
}
