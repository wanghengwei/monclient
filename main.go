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

	// args
	runAsDaemon = flag.Bool("d", false, "as daemon")
)

type App struct {
}

func (app *App) Run() error {
	pm := proc.NewProcessMonitor()

	// get stat data
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
						netRecv.WithLabelValues(proc.Command, strconv.Itoa(proc.PID), strconv.Itoa(l.Port)).Set(float64(l.Bytes))
					}
				}
			}

			time.Sleep(10 * time.Second)
		}
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
