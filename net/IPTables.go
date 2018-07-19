package net

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"

	"github.com/wanghengwei/monclient/cmdutil"
	"github.com/wanghengwei/monclient/common"
)

// TrafficMonitor is tool for TrafficMonitor
type TrafficMonitor struct {
	inputs []*InputItem
}

// NewTrafficMonitor TODO
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{}
}

// ClearInputs todo
func (t *TrafficMonitor) ClearInputs() {
	t.inputs = nil
}

// AddInput todo
func (t *TrafficMonitor) AddInput(pid int, port int) {
	t.inputs = append(t.inputs, &InputItem{
		PID:   pid,
		Port:  port,
		ready: false,
	})
}

// InputItem represent a listening socket
type InputItem struct {
	PID   int
	Port  int
	Bytes uint64
	ready bool
}

// // TrafficMonitorEstablishedItem represent an outgoing socket
// type TrafficMonitorEstablishedItem struct {
// 	PID           int
// 	LocalPort     int
// 	RemoteAddress string
// 	RemotePort    int
// }

// Snap run command one time
func (t *TrafficMonitor) Snap() error {
	err := t.snapInput()
	if err != nil {
		return err
	}
	// t.refreshOutputChain()
	// t.snapInput()
	// t.snapOutput()

	return nil
}

// FindInputBytes todo
func (t *TrafficMonitor) FindInputBytes(pid int, port int) uint64 {
	for _, item := range t.inputs {
		if item.PID == pid && item.Port == port {
			return item.Bytes
		}
	}

	return 0
}

func listRules(chain string) ([]*cmdutil.CommandResultLine, error) {
	lines, err := cmdutil.RunCommand("iptables", "-x", "-n", "-v", "-L", chain, "--line-numbers")
	if err != nil {
		return nil, err
	}

	return lines, nil
}

func (t *TrafficMonitor) snapInput() error {
	lines, err := listRules("INPUT")
	if err != nil {
		return err
	}

	for _, item := range t.inputs {
		item.ready = false
	}

	// line is like this
	// 1 0 0 tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:1080 /* pid=10234 */
	toDel := []string{}

	for _, l := range lines {
		ruleNumber := l.GetField(0).String()
		port := l.GetField(10).FindSubmatch(`dpt:(\d+)`).AsInt()

		if port == 0 {
			// bad line , to be removed
			log.Printf("bad line, port is 0: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}

		if len(l.Fields) != 14 {
			log.Printf("line is not created by me: %s\n", l)
			continue
		}

		pid := l.GetField(12).FindSubmatch(`pid=(\d+)`).AsInt()
		if pid == 0 {
			// remove it
			log.Printf("not found pid: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}

		// the rule is not interested, remove it
		item := t.findInput(pid, port)
		if item == nil {
			log.Printf("remove unwanted item: pid=%d, port=%d", pid, port)
			toDel = append(toDel, ruleNumber)
			continue
		}

		// read the bytes stat
		log.Printf("set bytes. line: %s", l)
		item.Bytes, err = common.DataStrToBytes(l.GetField(2).String())
		if err != nil {
			log.Printf("convert recv %s to bytes failed\n", l.GetField(2))
		}
		item.ready = true
	}

	// delete unwanted rules
	for _, item := range toDel {
		exec.Command("iptables", "-D", "INPUT", item).Run()
	}

	// create rule for non-ready items
	for _, item := range t.inputs {
		if item.ready {
			continue
		}

		log.Printf("create rule: pid=%d, port=%d", item.PID, item.Port)
		exec.Command("iptables", "-I", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(item.Port), "-mcomment", "--comment", fmt.Sprintf("pid=%d", item.PID)).Run()
		item.ready = true
	}

	return nil
}

func (t *TrafficMonitor) findInput(pid int, port int) *InputItem {
	for _, item := range t.inputs {
		if item.PID == pid && item.Port == port {
			return item
		}
	}

	return nil
}
