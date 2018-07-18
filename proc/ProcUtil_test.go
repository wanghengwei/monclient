package proc

import (
	"testing"
)

func TestFindProcsByPattern(t *testing.T) {
	ps := NewProcessMonitor(`^/sbin/init`)
	err := ps.Snap()
	if err != nil {
		t.Error(err)
	}

	procs := ps.Procs // FindProcsByPattern(regexp.MustCompile())
	if len(procs) != 1 || procs[0].PID != 1 {
		t.Error(procs)
	}
}

func TestGetPortByPattern(t *testing.T) {
	pu := NewProcessMonitor(`nc`)
	// pu.Excludes(`\[.*\]`)
	err := pu.Snap()
	if err != nil {
		t.Error(err)
	}

	ps := pu.Procs //.FindProcsByPattern(regexp.MustCompile())
	for _, p := range ps {
		t.Logf("%v", p)
	}
}
