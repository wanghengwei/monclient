package cmdutil

import (
	"testing"
)

func TestRunEcho(t *testing.T) {
	lines, err := RunCommand("echo", "112233")
	if err != nil {
		t.Error(err)
	}

	if len(lines) != 1 {
		t.Error(lines)
	}

	if lines[0].String() != "112233" {
		t.Error(lines[0])
	}
}

func TestRunEchoFields(t *testing.T) {
	lines, err := RunCommand("echo", "1 0 23453 tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:1080 /* pid=10234 */")

	if err != nil {
		t.Error(err)
	}

	if len(lines) != 1 {
		t.Error(lines)
	}

	if lines[0].GetField(2).AsUInt64() != 23453 {
		t.Error()
	}

	if lines[0].GetField(10).FindSubmatch(`dpt:(\d+)`).AsInt() != 1080 {
		t.Error()
	}
}
