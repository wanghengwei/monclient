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
	// outputs []*OutputItem
}

// NewTrafficMonitor TODO
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{}
}

// ClearAll 清除当前记录的所有端口信息
func (t *TrafficMonitor) ClearAll() {
	t.inputs = nil
	// t.outputs = nil
}

// AddInput 添加一个监听端口
func (t *TrafficMonitor) AddInput(pid int, port int) {
	t.inputs = append(t.inputs, &InputItem{
		PID:   pid,
		Port:  port,
		ready: false,
	})
}

// AddOutput 添加一个连出端口
// port 表示目标端口
// func (t *TrafficMonitor) AddOutput(pid int, port int) {
// 	t.outputs = append(t.outputs, &OutputItem{
// 		PID:        pid,
// 		SourcePort: port,
// 		ready:      false,
// 	})
// }

// InputItem represent a listening socket
type InputItem struct {
	PID      int
	Port     int
	InBytes  uint64
	OutBytes uint64
	ready    bool
}

// OutputItem 表示一个向外的连接
// type OutputItem struct {
// 	PID int
// 	// SourceAddress string
// 	SourcePort int
// 	// TargetAddress string
// 	// TargetPort    int
// 	Bytes uint64
// 	ready bool
// }

// Snap run command one time
func (t *TrafficMonitor) Snap() error {
	err := t.snapInput("INPUT")
	if err != nil {
		return err
	}

	err = t.snapInput("OUTPUT")
	if err != nil {
		return err
	}

	// err = t.snapOutput()
	// if err != nil {
	// 	return err
	// }

	return nil
}

// FindInputTraffics 获得一个监听端口的流量总字节(in and out)
func (t *TrafficMonitor) FindInputTraffics(pid int, port int) (uint64, uint64) {
	for _, item := range t.inputs {
		if item.PID == pid && item.Port == port {
			return item.InBytes, item.OutBytes
		}
	}

	return 0, 0
}

// FindOutputBytes 获得一个对外连接的流量总字节
// func (t *TrafficMonitor) FindOutputBytes(pid int, port int) uint64 {
// 	for _, item := range t.outputs {
// 		if item.PID == pid && item.SourcePort == port {
// 			return item.Bytes
// 		}
// 	}

// 	return 0
// }

func listRules(chain string) ([]*cmdutil.CommandResultLine, error) {
	lines, err := cmdutil.RunCommand("iptables", "-x", "-n", "-v", "-L", chain, "--line-numbers")
	if err != nil {
		return nil, err
	}

	return lines, nil
}

// 获得监听端口的信息
func (t *TrafficMonitor) snapInput(chain string) error {
	// chain := "INPUT"

	lines, err := listRules(chain)
	if err != nil {
		return err
	}

	for _, item := range t.inputs {
		item.ready = false
	}

	// line 每行大概长这样
	// 1 0 0 tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:1080 /* pid=10234 */

	toDel := []string{}

	for _, l := range lines {
		if len(l.Fields) != 14 {
			log.Printf("line is not created by me: %s\n", l)
			continue
		}

		ruleNumber := l.GetField(0).String()
		// 找到端口。可能是目标也可能是源
		port := l.GetField(10).FindSubmatch(`[ds]pt:(\d+)`).AsInt()

		if port == 0 {
			// bad line , to be removed
			log.Printf("bad line, port is 0: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}

		pid := l.GetField(12).FindSubmatch(`pid=(\d+)`).AsInt()
		if pid == 0 {
			// remove it
			log.Printf("not found pid: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}

		// 找到这个监听端口的信息
		item := t.findInput(pid, port)
		if item == nil {
			log.Printf("remove unwanted item: num=%d, pid=%d, port=%d", ruleNumber, pid, port)
			toDel = append(toDel, ruleNumber)
			continue
		}

		// read the bytes stat
		bs, err := common.DataStrToBytes(l.GetField(2).String())
		if err != nil {
			log.Printf("convert recv %s to bytes failed\n", l.GetField(2))
			toDel = append(toDel, ruleNumber)
			continue
		}

		if chain == "INPUT" {
			item.InBytes = bs
		} else {
			item.OutBytes = bs
		}

		item.ready = true
	}

	// delete unwanted rules
	for _, item := range toDel {
		exec.Command("iptables", "-D", chain, item).Run()
	}

	// create rule for non-ready items
	for _, item := range t.inputs {
		if item.ready {
			continue
		}

		log.Printf("create rule in %s: pid=%d, port=%d", chain, item.PID, item.Port)

		var portArg string
		if chain == "INPUT" {
			portArg = "--dport"
		} else {
			portArg = "--sport"
		}

		exec.Command("iptables", "-I", chain, "-p", "tcp", portArg, strconv.Itoa(item.Port), "-mcomment", "--comment", fmt.Sprintf("pid=%d", item.PID)).Run()
		item.ready = true
	}

	return nil
}

// 获得向外的连接的信息
// func (t *TrafficMonitor) snapOutput() error {
// 	lines, err := listRules("OUTPUT")
// 	if err != nil {
// 		return err
// 	}

// 	for _, item := range t.outputs {
// 		item.ready = false
// 	}

// 	// line is like this
// 	// 1 0 0 tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:1080 /* pid=10234 */
// 	toDel := []string{}

// 	for _, l := range lines {
// 		if len(l.Fields) != 14 {
// 			log.Printf("line is not created by me: %s\n", l)
// 			continue
// 		}

// 		ruleNumber := l.GetField(0).String()
// 		port := l.GetField(10).FindSubmatch(`spt:(\d+)`).AsInt()

// 		if port == 0 {
// 			// bad line , to be removed
// 			log.Printf("bad line, port is 0: %s", l)
// 			toDel = append(toDel, ruleNumber)
// 			continue
// 		}

// 		// // 获得目标地址
// 		// addr := l.GetField(8).String()

// 		pid := l.GetField(12).FindSubmatch(`pid=(\d+)`).AsInt()
// 		if pid == 0 {
// 			// remove it
// 			log.Printf("not found pid: %s", l)
// 			toDel = append(toDel, ruleNumber)
// 			continue
// 		}

// 		// the rule is not interested, remove it
// 		item := t.findOutput(pid, port)
// 		if item == nil {
// 			log.Printf("remove unwanted item(output): pid=%d, port=%d", pid, port)
// 			toDel = append(toDel, ruleNumber)
// 			continue
// 		}

// 		// read the bytes stat
// 		log.Printf("set bytes. line: %s", l)
// 		item.Bytes, err = common.DataStrToBytes(l.GetField(2).String())
// 		if err != nil {
// 			log.Printf("convert send %s to bytes failed\n", l.GetField(2))
// 		}
// 		item.ready = true
// 	}

// 	// delete unwanted rules
// 	for _, item := range toDel {
// 		exec.Command("iptables", "-D", "OUTPUT", item).Run()
// 	}

// 	// create rule for non-ready items
// 	for _, item := range t.outputs {
// 		if item.ready {
// 			continue
// 		}

// 		log.Printf("create output rule: pid=%d, port=%d", item.PID, item.SourcePort)
// 		exec.Command("iptables", "-I", "OUTPUT", "-p", "tcp", "--sport", strconv.Itoa(item.SourcePort), "-mcomment", "--comment", fmt.Sprintf("pid=%d", item.PID)).Run()
// 		item.ready = true
// 	}

// 	return nil
// }

func (t *TrafficMonitor) findInput(pid int, port int) *InputItem {
	for _, item := range t.inputs {
		if item.PID == pid && item.Port == port {
			return item
		}
	}

	return nil
}

// 查询一条对外接口记录
// func (t *TrafficMonitor) findOutput(pid int, port int) *OutputItem {
// 	for _, item := range t.outputs {
// 		if item.PID == pid && item.SourcePort == port {
// 			return item
// 		}
// 	}

// 	return nil
// }
