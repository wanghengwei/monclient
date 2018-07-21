package proc

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
)

var (
	socketListenStringPattern      = regexp.MustCompile(`^(.*):(\d+)`)
	socketEstablishedStringPattern = regexp.MustCompile(`^(.*):(\d+)->(.*):(\d+)`)
)

// Proc 表示一个进程
type Proc struct {
	PID           int
	Command       string
	CPU           float32
	MemoryVirtual uint64
	// 表示正在监听的端口
	ListenPorts []*SocketListen
	// 表示对外的连接
	ClientConns []*ClientConnection
}

// AddListenPort 添加一个监听的端口信息
func (p *Proc) AddListenPort(port int) {
	for _, item := range p.ListenPorts {
		if item.Port == port {
			return
		}
	}

	p.ListenPorts = append(p.ListenPorts, &SocketListen{
		Port: port,
	})
}

func (p *Proc) isListenPort(port int) bool {
	for _, c := range p.ListenPorts {
		if c.Port == port {
			return true
		}
	}

	return false
}

// AddClientConnections 增加一个对外的连接的信息
func (p *Proc) AddClientConnection(addr string, port int) {
	for _, i := range p.ClientConns {
		if i.Address == addr && i.Port == port {
			return
		}
	}

	log.Printf("add an output connection: %s:%d\n", addr, port)

	p.ClientConns = append(p.ClientConns, &ClientConnection{
		Address: addr,
		Port:    port,
	})
}

// SocketListen 表示监听的socket
type SocketListen struct {
	// 端口
	Port int
	// 流量总计，单位字节
	InBytes  uint64
	OutBytes uint64
}

// NewSocketListenByString 通过lsof的输出文本来创建一个监听socket
func NewSocketListenByString(line string) *SocketListen {
	ss := socketListenStringPattern.FindStringSubmatch(line)
	if ss == nil {
		return nil
	}

	port, err := strconv.Atoi(ss[2])
	if err != nil {
		return nil
	}

	return &SocketListen{
		// Address: ss[1],
		Port: port,
	}
}

func (s *SocketListen) String() string {
	return fmt.Sprintf(":%d (in:%d, out:%d)", s.Port, s.InBytes, s.OutBytes)
}

// ClientConnection 表示一个对外连接
type ClientConnection struct {
	Address string
	Port    int
	Bytes   uint64
}
