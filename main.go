package main

import (
	"flag"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/sevlyar/go-daemon"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wanghengwei/monclient/proc"
)

var (
	cpu = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "cpu_usage",
		Help:      "CPU Usage",
	}, []string{"cmd", "pid"})

	mem = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "mem_virt",
		Help:      "Memory Usage",
	}, []string{"cmd", "pid"})

	netRecv = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "net_recv",
		Help:      "Received Bytes",
	}, []string{"cmd", "pid", "port"})

	// 发送的字节数
	netSendFrom = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "net_sendfrom",
		Help:      "send bytes from local port",
	}, []string{"cmd", "pid", "port"})

	// 向某个远程地址发送的字节数
	netSendTo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "net_sendto",
		Help:      "send bytes to remote address",
	}, []string{"cmd", "pid", "addr", "port"})

	// 收的event
	eventRecv = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "x51",
		Name:      "event_recv",
		Help:      "Received Events",
	}, []string{"service", "pid", "id"})

	// args
	runAsDaemon = flag.Bool("d", false, "as daemon")

	// config
	config = &Config{}
)

type Config struct {
	PortBlacklist struct {
		Local  []string
		Remote []string
	}
}

// App 总入口
type App struct{}

func (app *App) applyConfig() {

}

// Run 执行主任务。不会返回
func (app *App) Run() error {

	config.PortBlacklist.Local = []string{"27151-27911"}
	config.PortBlacklist.Remote = []string{"27151-27911"}

	// 首先应用一次配置
	app.applyConfig()

	pm := proc.NewProcessMonitor()

	// 获得cpu、mem等数据，这些数据来源于周期性的执行系统命令，比如ps
	go func() {
		for {
			// 每次循环开头都应用下配置，因为配置可能会运行时刷新
			app.applyConfig()
			// 设置本地端口黑名单
			pm.ClearBlacklist()
			setPortBlacklist(config.PortBlacklist.Local, pm.AddSinglePortToLocalBlacklist, pm.AddPortRangeToLocalBlacklist)
			setPortBlacklist(config.PortBlacklist.Remote, pm.AddSinglePortToRemoteBlacklist, pm.AddPortRangeToRemoteBlacklist)

			log.Printf("snapping...\n")
			err := pm.Snap()
			if err != nil {
				log.Println(err)
			} else {
				for _, proc := range pm.Procs {
					cpu.WithLabelValues(proc.Command, strconv.Itoa(proc.PID)).Set(float64(proc.CPU))
					mem.WithLabelValues(proc.Command, strconv.Itoa(proc.PID)).Set(float64(proc.MemoryVirtual))
					for _, l := range proc.ListenPorts {
						netRecv.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), strconv.Itoa(l.Port)).Set(float64(l.InBytes))
						netSendFrom.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), strconv.Itoa(l.Port)).Set(float64(l.OutBytes))
					}
					for _, c := range proc.ClientConns {
						netSendTo.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), c.Address, strconv.Itoa(c.Port)).Set(float64(c.Bytes))
					}
				}
			}

			time.Sleep(10 * time.Second)
		}
	}()

	// 通过log来分析event数量
	go func() {

	}()

	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(":10001", nil)
}

func main() {
	flag.Parse()

	// 这段if是为了用daemon方式运行
	if *runAsDaemon {
		ctx := daemon.Context{
			PidFileName: "/tmp/monclient.pid",
			WorkDir:     "/tmp",
		}
		d, err := ctx.Reborn()
		if err != nil {
			log.Fatal(err)
		}
		if d != nil {
			return
		}
		defer ctx.Release()
	}

	// 启动应用
	app := App{}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func setPortBlacklist(ports []string, f1 func(int), f2 func(int, int)) {
	numberRe := regexp.MustCompile(`^(\d+)$`)
	rangeRe := regexp.MustCompile(`^(\d+)-(\d+)$`)
	for _, s := range ports {
		if ss := numberRe.FindStringSubmatch(s); len(ss) > 0 {
			port, _ := strconv.Atoi(ss[1])
			f1(port)
		} else if ss := rangeRe.FindStringSubmatch(s); len(ss) > 0 {
			a, _ := strconv.Atoi(ss[1])
			b, _ := strconv.Atoi(ss[2])
			f2(a, b)
		}
	}
}
