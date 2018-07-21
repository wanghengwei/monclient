package lsof

import (
	"testing"
)

func TestRun(t *testing.T) {
	lsof := &Lsof{}
	r, err := lsof.Run()
	if err != nil {
		t.Error(err)
	}

	t.Logf("found %d items\n", len(r.items))
	for _, item := range r.items {
		t.Logf("%v\n", item)
	}
}
