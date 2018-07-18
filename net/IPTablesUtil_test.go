package net

import (
	"testing"
)

func TestGetTrafficOfNetwork(t *testing.T) {
	m := NewTrafficMonitor()
	m.ClearInputs()
	m.AddInput(2519, 1080)
	err := m.Snap()
	if err != nil {
		t.Error(err)
	}

	for _, item := range m.inputs {
		t.Logf("%v", item)
	}
}
