package main

import (
	"flag"
	"log"
	"net/http"
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
	netSend = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "x51",
		Name:      "net_send",
		Help:      "Sent Bytes",
	}, []string{"cmd", "pid", "port"})

	// 收的event
	eventRecv = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "x51",
		Name:      "event_recv",
		Help:      "Received Events",
	}, []string{"service", "pid", "id"})

	// args
	runAsDaemon = flag.Bool("d", false, "as daemon")
)

type App struct {
}

func (app *App) Run() error {
	pm := proc.NewProcessMonitor()

	// 获得cpu、mem等数据，这些数据来源于周期性的执行系统命令，比如ps
	go func() {
		for {
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
						netSend.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), strconv.Itoa(l.Port)).Set(float64(l.OutBytes))
					}
					// for _, l := range proc.EstablishedSockets {
					// 	netSend.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), l.TargetAddress, strconv.Itoa(l.TargetPort)).Set(float64(l.Bytes))
					// }
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

	app := App{}
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
