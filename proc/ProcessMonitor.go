package proc

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/wanghengwei/monclient/cmdutil"
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
	lines, err := cmdutil.RunCommand("lsof", "-a", "-iTCP", "-P")
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
		} else if st == "(ESTABLISHED)" {
			sock := NewSocketEstablishedByString(name)
			if sock != nil {
				// TODO
				// proc.EstablishedConnections = append(proc.EstablishedConnections, sock)
			}
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
		proc.MemoryVirtual, _ = memStrToUInt64Byte(line.GetField(4).String())
	}

	return nil
}

func (p *ProcessMonitor) snapByTrafficMonitor() error {
	p.trafficMonitor.ClearInputs()
	for _, proc := range p.Procs {
		for _, l := range proc.ListenPorts {
			p.trafficMonitor.AddInput(proc.PID, l.Port)
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
			l.Bytes = p.trafficMonitor.FindInputBytes(proc.PID, l.Port)
		}
	}

	return nil
}

// 检查一个命令行是否应当被记录。判断条件包括includes条件和exludes条件。
func (p *ProcessMonitor) matchCommand(c string) bool {
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

func memStrToUInt64Byte(s string) (uint64, error) {
	m, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		if m < 0 {
			return 0, fmt.Errorf("negative mem: %s", s)
		}
		return m, nil
	}

	re := regexp.MustCompile(`(\d+(?:\.\d+)?)([kmg])`)
	ss := re.FindStringSubmatch(s)
	if ss == nil {
		return 0, fmt.Errorf("wrong format: %s", s)
	}

	n, err := strconv.ParseFloat(ss[1], 64)
	if err != nil {
		return 0, err
	}

	if n < 0 {
		return 0, fmt.Errorf("negative mem: %s", s)
	}

	var bytes uint64
	u := ss[2]
	if u == "k" {
		bytes = uint64(n * 1024)
	} else if u == "m" {
		bytes = uint64(n * 1024 * 1024)
	} else if u == "g" {
		bytes = uint64(n * 1024 * 1024 * 1024)
	} else {
		return 0, fmt.Errorf("invalid unit: %s", u)
	}

	return bytes, nil
}
