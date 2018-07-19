package proc

import (
	"fmt"
	"log"
	"regexp"

	"github.com/wanghengwei/monclient/cmdutil"
	"github.com/wanghengwei/monclient/common"
	"github.com/wanghengwei/monclient/net"
)

// ProcessMonitor is a util for process
// example:
// u := NewProcessMonitor()
// u.Snap()
type ProcessMonitor struct {
	Procs []*Proc

	filters  []*regexp.Regexp
	excludes []*regexp.Regexp

	trafficMonitor *net.TrafficMonitor
}

// NewProcessMonitor create a ProcessMonitor object
func NewProcessMonitor(filters ...string) *ProcessMonitor {
	p := &ProcessMonitor{}
	for _, f := range filters {
		p.filters = append(p.filters, regexp.MustCompile(f))
	}
	p.trafficMonitor = net.NewTrafficMonitor()
	return p
}

// Includes todo
func (p *ProcessMonitor) Includes(pattern ...string) {
	for _, pt := range pattern {
		p.filters = append(p.filters, regexp.MustCompile(pt))
	}
}

// Excludes todo
func (p *ProcessMonitor) Excludes(pattern ...string) {
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
	lines, err := cmdutil.RunCommand("lsof", "-a", "-iTCP", "-P", "-n")
	if err != nil {
		// lsof出错不是很重要，就是没了端口信息而已，忽视
		// return fmt.Errorf("run lsof failed: %s", err)
		return nil
	}

	for _, line := range lines {
		pid := line.GetField(1).AsInt()
		if pid == 0 {
			log.Printf("skip invalid line of lsof: %s\n", line)
			continue
		}

		proc := p.FindProcByPID(pid)
		if proc == nil {
			log.Printf("Cannot find proc by pid: %d", pid)
			continue
		}

		var name string
		var st string
		if len(line.Fields) == 10 {
			st = line.GetField(9).String()
			name = line.GetField(8).String()
		} else if len(line.Fields) == 9 {
			st = line.GetField(8).String()
			name = line.GetField(7).String()
		}

		if st == "(LISTEN)" {
			listen := NewSocketListenByString(name)
			if listen != nil {
				proc.AddListenPort(listen)
			}
			// } else if st == "(ESTABLISHED)" {
			// 	sock := NewSocketEstablishedByString(name)
			// 	if sock != nil {
			// 		proc.AddEstablishedSocket(sock)
			// 	}
		} else {
			log.Printf("Unknown lsof name: %s", st)
		}
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
		// for _, l := range proc.ListenPorts {
		// 	p.trafficMonitor.AddOutput(proc.PID, l.Port)
		// }
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

		// for _, l := range proc.EstablishedSockets {
		// 	l.Bytes = p.trafficMonitor.FindOutputBytes(proc.PID, l.SourcePort)
		// }
	}

	return nil
}

// 检查一个命令行是否应当被记录。判断条件包括includes条件和exludes条件。
func (p *ProcessMonitor) matchCommand(c string) bool {
	// 先排除一些内定的
	if c == "ps -ef" {
		return false
	}

	if matched, _ := regexp.MatchString(`^\[.+\]`, c); matched {
		return false
	}

	// 暂时只看service_box的
	// if matched, _ := regexp.MatchString(`service_box`, c); !matched {
	// 	return false
	// }

	matched := false
	if len(p.filters) > 0 {
		for _, r := range p.filters {
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
