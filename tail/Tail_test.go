package tail

import (
	"log"
	"testing"
)

func TestTail(t *testing.T) {
	tm, err := New("/tmp", `.*\.txt`)
	if err != nil {
		t.Error(err)
	}

	for {
		d := <-tm.NewData
		log.Printf("new line: %s in file: %s\n", d.Line, d.FilePath)
	}
}
