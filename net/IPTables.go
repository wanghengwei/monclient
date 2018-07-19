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
	inputs            []*InputItem
	clientConnections []*ClientConnection
}

// NewTrafficMonitor TODO
func NewTrafficMonitor() *TrafficMonitor {
	return &TrafficMonitor{}
}

// ClearAll 清除当前记录的所有端口信息
func (t *TrafficMonitor) ClearAll() {
	t.inputs = nil
	t.clientConnections = nil
}

// AddInput 添加一个需要统计的监听端口
// 这个端口会被加到INPUT/OUTPUT chain里
func (t *TrafficMonitor) AddInput(pid int, port int) {
	t.inputs = append(t.inputs, &InputItem{
		PID:   pid,
		Port:  port,
		ready: false,
	})
}

// AddClientConnection 添加一个作为客户端连出去的iptables rule
// port 表示远程目标端口
func (t *TrafficMonitor) AddClientConnection(pid int, addr string, port int) {
	t.clientConnections = append(t.clientConnections, &ClientConnection{
		PID:     pid,
		Address: addr,
		Port:    port,
		ready:   false,
	})
}

// InputItem represent a listening socket
type InputItem struct {
	PID      int
	Port     int
	InBytes  uint64
	OutBytes uint64
	ready    bool
}

// ClientConnection 表示作为客户端向外的连接，不包括监听端口对外发包的方向
type ClientConnection struct {
	PID int
	// 远端的地址，比如ip
	Address string
	// 远端目标端口
	Port int
	// 发送的字节数
	Bytes uint64
	ready bool
}

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

	err = t.snapClient()
	if err != nil {
		return err
	}

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
func (t *TrafficMonitor) FindClientOutput(pid int, addr string, port int) uint64 {
	for _, item := range t.clientConnections {
		if item.PID == pid && item.Port == port && item.Address == addr {
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
	// 1 0 0 tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:1080 /* pid=10234;type=server */

	toDel := []string{}

	for _, l := range lines {
		if len(l.Fields) != 14 {
			log.Printf("line is not created by me: %s\n", l)
			continue
		}

		ruleNumber := l.GetField(0).String()

		tmp, err := l.GetField(12).FindSubmatches(`pid=(\d+);type=(.*)`)
		if err != nil || len(tmp) != 3 {
			// remove it
			log.Printf("cannot find server rule: line=%s, err=%s\n", l, err)
			toDel = append(toDel, ruleNumber)
			continue
		}
		if tmp[2] != "server" {
			// 不是监听的端口的rule，跳过
			log.Printf("type is not server, skip\n")
			continue
		}

		pid, err := strconv.Atoi(tmp[1])
		if err != nil {
			toDel = append(toDel, ruleNumber)
			continue
		}

		// 找到端口。可能是目标也可能是源
		var pt string
		if chain == "INPUT" {
			pt = `dpt:(\d+)`
		} else {
			// 对外则作为源端口
			pt = `spt:(\d+)`
		}
		port := l.GetField(10).FindSubmatch(pt).AsInt()

		if port == 0 {
			// bad line , to be removed
			log.Printf("bad line, port is 0: %s", l)
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

		exec.Command("iptables", "-I", chain, "-p", "tcp", portArg, strconv.Itoa(item.Port), "-mcomment", "--comment", fmt.Sprintf("pid=%d;type=server", item.PID)).Run()
		item.ready = true
	}

	return nil
}

// 获得向外的连接的信息
func (t *TrafficMonitor) snapClient() error {
	lines, err := listRules("OUTPUT")
	if err != nil {
		return err
	}

	for _, item := range t.clientConnections {
		item.ready = false
	}

	// line is like this
	// 1 0 0 tcp -- * * 0.0.0.0/0 1.2.3.4 tcp dpt:1080 /* pid=10234;type=client */

	toDel := []string{}

	for _, l := range lines {
		if len(l.Fields) != 14 {
			log.Printf("line is not created by me: %s\n", l)
			continue
		}

		ruleNumber := l.GetField(0).String()

		tmp, err := l.GetField(12).FindSubmatches(`pid=(\d+);type=(.*)`)
		if err != nil || len(tmp) != 3 {
			// remove it
			log.Printf("cannot find client comment in line: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}
		if tmp[2] != "client" {
			// 不是监听的端口的rule，跳过
			continue
		}

		pid, err := strconv.Atoi(tmp[1])
		if err != nil {
			toDel = append(toDel, ruleNumber)
			continue
		}

		// 目标地址
		addr := l.GetField(8).String()

		// 目标端口，因此是dpt
		port := l.GetField(10).FindSubmatch(`dpt:(\d+)`).AsInt()

		if port == 0 {
			// bad line , to be removed
			log.Printf("bad line, port is 0: %s", l)
			toDel = append(toDel, ruleNumber)
			continue
		}

		item := t.findClient(pid, addr, port)
		// the rule is not interested, remove it
		if item == nil {
			log.Printf("remove unwanted item(output): pid=%d, port=%d", pid, port)
			toDel = append(toDel, ruleNumber)
			continue
		}

		// read the bytes stat
		log.Printf("set bytes. line: %s", l)
		item.Bytes, err = common.DataStrToBytes(l.GetField(2).String())
		if err != nil {
			log.Printf("convert send %s to bytes failed\n", l.GetField(2))
			toDel = append(toDel, ruleNumber)
			continue
		}

		item.ready = true
	}

	// delete unwanted rules
	for _, item := range toDel {
		exec.Command("iptables", "-D", "OUTPUT", item).Run()
	}

	// create rule for non-ready items
	for _, item := range t.clientConnections {
		if item.ready {
			continue
		}

		log.Printf("create output rule: pid=%d, addr=%s, port=%d", item.PID, item.Address, item.Port)

		exec.Command("iptables", "-I", "OUTPUT", "-p", "tcp", "-d", item.Address, "--dport", strconv.Itoa(item.Port), "-mcomment", "--comment", fmt.Sprintf("pid=%d;type=client", item.PID)).Run()

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

// 查询一条对外接口记录
func (t *TrafficMonitor) findClient(pid int, addr string, port int) *ClientConnection {
	for _, item := range t.clientConnections {
		if item.PID == pid && item.Address == addr && item.Port == port {
			return item
		}
	}

	return nil
}
