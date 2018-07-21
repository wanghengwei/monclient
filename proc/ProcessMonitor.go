package proc

import (
	"fmt"
	"log"
	"regexp"

	"github.com/wanghengwei/monclient/cmdutil"
	"github.com/wanghengwei/monclient/common"
	"github.com/wanghengwei/monclient/lsof"
	"github.com/wanghengwei/monclient/net"
)

// ProcessMonitor is a util for process
// example:
// u := NewProcessMonitor()
// u.Snap()
type ProcessMonitor struct {
	Procs []*Proc

	includes []*regexp.Regexp
	excludes []*regexp.Regexp

	trafficMonitor *net.TrafficMonitor

	blacklistLocal  []func(int) bool
	blacklistRemote []func(int) bool
}

// NewProcessMonitor create a ProcessMonitor object
func NewProcessMonitor(includes ...string) *ProcessMonitor {
	p := &ProcessMonitor{}
	for _, f := range includes {
		p.includes = append(p.includes, regexp.MustCompile(f))
	}
	p.trafficMonitor = net.NewTrafficMonitor()
	return p
}

// AddSinglePortToLocalBlacklist 添加一个本地端口到黑名单
func (p *ProcessMonitor) AddSinglePortToLocalBlacklist(port int) {
	f := func(x int) bool {
		return x == port
	}
	p.blacklistLocal = append(p.blacklistLocal, f)
}

// AddSinglePortToRemoteBlacklist 添加一个本地端口到黑名单
func (p *ProcessMonitor) AddSinglePortToRemoteBlacklist(port int) {
	f := func(x int) bool {
		return x == port
	}
	p.blacklistRemote = append(p.blacklistRemote, f)
}

// AddPortRangeToLocalBlacklist 添加一个本地端口范围到黑名单。包括两端
func (p *ProcessMonitor) AddPortRangeToLocalBlacklist(from int, to int) {
	f := func(x int) bool {
		return x >= from && x <= to
	}
	p.blacklistLocal = append(p.blacklistLocal, f)
}

func (p *ProcessMonitor) AddPortRangeToRemoteBlacklist(from int, to int) {
	f := func(x int) bool {
		return x >= from && x <= to
	}
	p.blacklistRemote = append(p.blacklistRemote, f)
}

func (p *ProcessMonitor) ClearBlacklist() {
	p.blacklistLocal = nil
	p.blacklistRemote = nil
	p.includes = nil
	p.excludes = nil
}

func (p *ProcessMonitor) inBlacklistOfLocal(port int) bool {
	for _, f := range p.blacklistLocal {
		b := f(port)
		if b {
			return true
		}
	}

	return false
}

func (p *ProcessMonitor) inBlacklistOfRemote(port int) bool {
	for _, f := range p.blacklistRemote {
		b := f(port)
		if b {
			return true
		}
	}

	return false
}

// Includes todo
func (p *ProcessMonitor) AddIncludes(pattern ...string) {
	for _, pt := range pattern {
		p.includes = append(p.includes, regexp.MustCompile(pt))
	}
}

// Excludes todo
func (p *ProcessMonitor) AddExcludes(pattern ...string) {
	for _, pt := range pattern {
		p.excludes = append(p.excludes, regexp.MustCompile(pt))
	}
}

func (p *ProcessMonitor) snapByPS() error {
	// 执行ps获得进程基本信息
	c := cmdutil.NewCommand("ps", "-ef")
	c.SplitNumber = 8
	lines, err := c.Run()
	if err != nil {
		return fmt.Errorf("run ps failed: %s", err)
	}

	// 在刷新数据前清除掉老的数据
	p.Procs = nil

	for _, line := range lines[1:] {
		item := new(Proc)
		item.PID = line.GetField(1).AsInt()
		if item.PID == 0 {
			log.Printf("skip invalid line of ps: %s\n", line)
			continue
		}

		item.Command = line.GetField(7).String()
		if p.matchCommand(item.Command) {
			p.Procs = append(p.Procs, item)
		}
	}

	return nil
}

func (p *ProcessMonitor) snapByLSOF() error {
	// lines, err := cmdutil.RunCommand("lsof", "-a", "-iTCP", "-P", "-n")
	lsof := &lsof.Lsof{}
	result, err := lsof.Run()
	if err != nil {
		// lsof出错不是很重要，就是没了端口信息而已，忽视
		return nil
	}

	// clientConnLines := []*cmdutil.CommandResultLine{}

	for _, item := range result.GetListenItems() {
		// pid := line.GetField(1).AsInt()
		// if pid == 0 {
		// 	log.Printf("skip unknown line of lsof: %s\n", line)
		// 	continue
		// }

		proc := p.FindProcByPID(item.PID)
		if proc == nil {
			log.Printf("cannot find process by pid %d, skip", item.PID)
			continue
		}

		// var name string
		// var st string
		// if len(line.Fields) == 10 {
		// 	st = line.GetField(9).String()
		// 	name = line.GetField(8).String()
		// } else if len(line.Fields) == 9 {
		// 	st = line.GetField(8).String()
		// 	name = line.GetField(7).String()
		// }

		// if st == "(LISTEN)" {
		// 	// 找出所有监听的端口
		// listen := NewSocketListenByString(name)

		// 看看是不是在黑名单里，在就忽略
		if p.inBlacklistOfLocal(item.BindPort) {
			log.Printf("the local port %d is in blacklist, skip\n", item.BindPort)
			continue
		}

		// if listen != nil {
		proc.AddListenPort(item.BindPort)
		// }
		// } else if st == "(ESTABLISHED)" {
		// 	// client socket，name形如 src:spt->dst:dpt 这种格式。
		// 	// src和spt不重要，不过要排除spt已经是一个监听端口的情况，因为在上面那个if分支
		// 	// 里已经处理了。

		// 	// 将这个端口暂存，等循环完再添加，因为要先搞定监听的端口
		// 	clientConnLines = append(clientConnLines, line)

		// } else {
		// 	log.Printf("Unknown lsof name: %s", st)
		// }
	}

	// 最后添加client连接，这是为了先把监听的端口搞定
	for _, item := range result.GetEstablishedItems() {
		// pid := line.GetField(1).AsInt()
		// if pid == 0 {
		// 	log.Printf("skip unknown line of lsof: %s\n", line)
		// 	continue
		// }

		proc := p.FindProcByPID(item.PID)
		if proc == nil {
			log.Printf("cannot find process by pid %d, skip", item.PID)
			continue
		}

		// var name string
		// if len(line.Fields) == 10 {
		// 	name = line.GetField(8).String()
		// } else if len(line.Fields) == 9 {
		// 	name = line.GetField(7).String()
		// }

		// re := regexp.MustCompile(`^(.*):(\d+)->(.*):(\d+)`)
		// ts := re.FindStringSubmatch(name)
		// if ts == nil {
		// 	// 没找到不太正常，跳过算了
		// 	log.Printf("cannot find format src:spt->dst:dpt in %s\n", name)
		// 	continue
		// }

		// // 看看源端口是不是一个监听的端口
		// spt, err := strconv.Atoi(ts[2])
		// if err != nil {
		// 	log.Printf("cannot extract spt as int from %s\n", name)
		// 	continue
		// }
		if proc.isListenPort(item.SourcePort) {
			log.Printf("%d is a listen port, skip\n", item.SourcePort)
			continue
		}
		// 检查源端口是不是在黑名单
		if p.inBlacklistOfLocal(item.SourcePort) {
			log.Printf("the local port %d is in blacklist, skip\n", item.SourcePort)
			continue
		}

		// // 找出目标端口
		// dpt, err := strconv.Atoi(ts[4])
		// if err != nil {
		// 	log.Printf("%s is not valid int port\n", ts[4])
		// 	continue
		// }
		// 检查黑名单
		if p.inBlacklistOfRemote(item.TargetPort) {
			log.Printf("the remote port %d is in blacklist, skip\n", item.TargetPort)
			continue
		}
		// conn := &ClientConnection{ts[3], dpt, 0}

		proc.AddClientConnection(item.TargetAddress, item.TargetPort)
	}

	return nil
}

func (p *ProcessMonitor) snapByTop() error {
	cmd := cmdutil.NewCommand("top", "-b", "-n", "1")
	cmd.SplitNumber = 12
	cmd.IgnoreExitCode = true

	lines, err := cmd.Run()
	if err != nil {
		return fmt.Errorf("run top failed: %s", err)
	}

	for _, line := range lines {
		if len(line.Fields) != 12 {
			log.Printf("drop unwanted line: %s", line.String())
			continue
		}

		pid := line.GetField(0).AsInt()
		if pid == 0 {
			continue
		}

		proc := p.FindProcByPID(pid)
		if proc == nil {
			continue
		}

		proc.CPU = line.GetField(8).AsFloat32()
		proc.MemoryVirtual, err = common.DataStrToBytes(line.GetField(4).String())
		if err != nil {
			log.Printf("convert mem %s to bytes failed\n", line.GetField(4))
		}
	}

	return nil
}

func (p *ProcessMonitor) snapByTrafficMonitor() error {
	p.trafficMonitor.ClearAll()
	for _, proc := range p.Procs {
		for _, l := range proc.ListenPorts {
			p.trafficMonitor.AddInput(proc.PID, l.Port)
		}
		for _, c := range proc.ClientConns {
			p.trafficMonitor.AddClientConnection(proc.PID, c.Address, c.Port)
		}
	}

	err := p.trafficMonitor.Snap()
	if err != nil {
		// iptables 失败，不是很要紧，多半是没用root跑。
		log.Printf("TrafficMonitor.Snap FAILED: %s\n", err)
		// return err
	}

	for _, proc := range p.Procs {
		for _, l := range proc.ListenPorts {
			l.InBytes, l.OutBytes = p.trafficMonitor.FindInputTraffics(proc.PID, l.Port)
		}

		for _, l := range proc.ClientConns {
			l.Bytes = p.trafficMonitor.FindClientOutput(proc.PID, l.Address, l.Port)
		}
	}

	return nil
}

// 检查一个命令行是否应当被记录。判断条件包括includes条件和exludes条件。
func (p *ProcessMonitor) matchCommand(c string) bool {
	// 先排除一些内定的
	if c == "ps -ef" {
		return false
	}

	// if matched, _ := regexp.MatchString(`^\[.+\]`, c); matched {
	// 	return false
	// }

	// 暂时只看service_box的
	// if matched, _ := regexp.MatchString(`service_box`, c); !matched {
	// 	return false
	// }

	matched := false
	if len(p.includes) > 0 {
		for _, r := range p.includes {
			if len(r.FindString(c)) != 0 {
				matched = true
				break
			}
		}
	} else {
		matched = true
	}

	if !matched {
		return false
	}

	excluded := false
	for _, r := range p.excludes {
		if len(r.FindString(c)) != 0 {
			excluded = true
			break
		}
	}

	if excluded {
		return false
	}

	return true
}

// FindProcByPID find proccess by pid. return nil if not found
func (p *ProcessMonitor) FindProcByPID(pid int) *Proc {
	for _, proc := range p.Procs {
		if proc.PID == pid {
			return proc
		}
	}

	return nil
}

// Snap snap info by calling system command, ps/lsof etc.
func (p *ProcessMonitor) Snap() error {
	log.Printf("snap by ps...")
	err := p.snapByPS()
	log.Printf("snap by ps DONE")
	if err != nil {
		return err
	}

	log.Printf("snap by lsof...")
	err = p.snapByLSOF()
	log.Printf("snap by lsof DONE")
	if err != nil {
		return err
	}

	log.Printf("snap by top...")
	err = p.snapByTop()
	log.Printf("snap by top DONE")
	if err != nil {
		return err
	}

	log.Printf("snap by trafficmonitor...")
	err = p.snapByTrafficMonitor()
	log.Printf("snap by trafficmonitor DONE")
	if err != nil {
		return err
	}

	return nil
}

// FindProcsByPattern find process by command pattern
func (p *ProcessMonitor) FindProcsByPattern(pattern *regexp.Regexp) []*Proc {
	results := []*Proc{}

	for _, item := range p.Procs {
		if len(pattern.FindString(item.Command)) != 0 {
			results = append(results, item)
		}
	}

	return results
}
