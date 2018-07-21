package lsof

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/wanghengwei/monclient/cmdutil"
)

var (
	listenRe      = regexp.MustCompile(`^(.*):(\d+)$`)
	establishedRe = regexp.MustCompile(`^(.*):(\d+)->(.*):(\d+)$`)
)

const (
	Listen SocketType = iota
	Established
)

type SocketType int

type Lsof struct{}

func (l *Lsof) Run() (*Result, error) {
	cmd := cmdutil.NewCommand("lsof", "-a", "-n", "-P", "-i4TCP")
	lines, err := cmd.Run()
	if err != nil {
		return nil, err
	}

	rez := &Result{}

	for _, line := range lines {
		pid := line.GetField(1).AsInt()
		// pid 不能为0
		if pid <= 0 {
			log.Printf("skip line which cannot extract pid from field 1: %s\n", line)
			continue
		}

		n := len(line.Fields)

		connType := line.GetField(n - 1).String()
		nameField := line.GetField(n - 2).String()

		var item Item

		switch connType {
		case "(LISTEN)":
			ms := listenRe.FindStringSubmatch(nameField)
			if ms == nil {
				log.Printf("cannot find src:port from field %s\n", nameField)
				continue
			}

			port, _ := strconv.Atoi(ms[2])

			item = &ListenItem{
				BaseItem:    BaseItem{pid},
				BindAddress: ms[1],
				BindPort:    port,
			}

			break
		case "(ESTABLISHED)":
			ms := establishedRe.FindStringSubmatch(nameField)
			if ms == nil {
				log.Printf("cannot find established pattern from %s\n", nameField)
				continue
			}

			sport, _ := strconv.Atoi(ms[2])
			dport, _ := strconv.Atoi(ms[4])

			item = &EstablishedItem{
				BaseItem:      BaseItem{pid},
				SourceAddress: ms[1],
				SourcePort:    sport,
				TargetAddress: ms[3],
				TargetPort:    dport,
			}

			break
		default:
			log.Printf("unsupported type: %s\n", connType)
			continue
		}
		rez.items = append(rez.items, item)
	}

	return rez, nil
}

type Result struct {
	items []Item
}

func (r *Result) GetListenItems() []*ListenItem {
	rez := []*ListenItem{}

	for _, item := range r.items {
		if ri, ok := item.(*ListenItem); ok {
			rez = append(rez, ri)
		}
	}

	return rez
}

func (r *Result) GetEstablishedItems() []*EstablishedItem {
	rez := []*EstablishedItem{}

	for _, item := range r.items {
		if ri, ok := item.(*EstablishedItem); ok {
			rez = append(rez, ri)
		}
	}

	return rez
}

type Item interface{}

type BaseItem struct {
	PID int
}

type ListenItem struct {
	BaseItem
	BindAddress string
	BindPort    int
}

func (li *ListenItem) String() string {
	return fmt.Sprintf("%d: %s:%d", li.PID, li.BindAddress, li.BindPort)
}

type EstablishedItem struct {
	BaseItem
	SourceAddress string
	SourcePort    int
	TargetAddress string
	TargetPort    int
}

func (ei *EstablishedItem) String() string {
	return fmt.Sprintf("%d: %s:%d->%s:%d", ei.PID, ei.SourceAddress, ei.SourcePort, ei.TargetAddress, ei.TargetPort)
}
