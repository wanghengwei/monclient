package x51log

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/wanghengwei/monclient/tail"
)

var (
	x51EventLogFileNameRe = regexp.MustCompile(`^(.+)_stat_(send|recv)event_.*\.\d+_\d+_\d+_(\d+).*\.txt$`)
)

type X51EventLogCollector struct {
	logFolder          string
	sendEventCountFunc func(string, int, string, int)
	sendEventSizeFunc  func(string, int, string, int)
	recvEventCountFunc func(string, int, string, int)
	recvEventSizeFunc  func(string, int, string, int)
}

func NewX51EventLogCollector(folder string, scf, ssf, rcf, rsf func(string, int, string, int)) *X51EventLogCollector {
	return &X51EventLogCollector{
		logFolder:          folder,
		sendEventCountFunc: scf,
		sendEventSizeFunc:  ssf,
		recvEventCountFunc: rcf,
		recvEventSizeFunc:  rsf,
	}
}

func (lc *X51EventLogCollector) Run() error {
	tm, err := tail.New(lc.logFolder, `^.+_stat_(?:send|recv)event_.+\.txt$`)
	if err != nil {
		return err
	}

	for {
		data, ok := <-tm.NewData
		if !ok {
			glog.Infof("channel of tail closed\n")
			break
		}

		glog.V(1).Infof("%s: %s\n", data.FilePath, data.Line)

		// 如果是空白行就跳过
		line := strings.TrimSpace(data.Line)
		if len(line) == 0 {
			glog.V(1).Infof("skip empty line: file=%s\n", data.FilePath)
			continue
		}

		// 从文件名获得一些信息
		fileName := path.Base(data.FilePath)
		ms := x51EventLogFileNameRe.FindStringSubmatch(fileName)
		if ms == nil {
			glog.Warningf("bad log file name: %s\n", fileName)
			continue
		}

		serviceName := ms[1]
		sendOrRecv := ms[2]
		pid, err := strconv.Atoi(ms[3])
		if err != nil {
			glog.Warningf("pid is not a number: %s\n", line)
			continue
		}

		// 分析每行以便获得消息和大小信息。每行有7列或6列
		cols := strings.Fields(line)
		if len(cols) < 6 {
			glog.Warningf("bad line of log: %s\n", line)
			continue
		}

		eventName := cols[3]
		count, err := strconv.Atoi(cols[4])
		if err != nil {
			glog.Warningf("event count is not a number: %s\n", line)
			continue
		}
		size, err := strconv.Atoi(cols[5])
		if err != nil {
			glog.Warningf("event size is not a number: %s\n", line)
			continue
		}

		// 记录统计数据
		switch sendOrRecv {
		case "send":
			lc.sendEventCountFunc(serviceName, pid, eventName, count)
			lc.sendEventSizeFunc(serviceName, pid, eventName, size)
			break
		case "recv":
			lc.recvEventCountFunc(serviceName, pid, eventName, count)
			lc.recvEventSizeFunc(serviceName, pid, eventName, size)
			break
		default:
			glog.Warningf("invalid log type: %s\n", sendOrRecv)
			break
		}
	}

	return nil
}
