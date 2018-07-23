package x51log

import (
	"testing"

	"github.com/golang/glog"
)

func TestX51EventLog(t *testing.T) {
	f := func(srv string, pid int, ev string, n int) {
		glog.Infof("%d %s %s: %d\n", pid, srv, ev, n)
	}
	elc := NewX51EventLogCollector("/tmp", f, f, f, f)
	err := elc.Run()
	if err != nil {
		t.Error(err)
	}
}
