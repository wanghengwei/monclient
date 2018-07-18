package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

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

	// config
	globalConfig = &Config{}

	// args
	configPath = flag.String("config", "", "")
)

// Config todo
type Config struct {
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
}

type SafeConfig struct {
	Data Config
	mux  sync.Mutex
}

func main() {
	flag.Parse()

	// read config from local file
	// var cp string

	// if len(*configPath) == 0 {
	// 	cp = filepath.Join(filepath.Dir(os.Args[0]), "config.json")
	// } else {
	// 	cp = *configPath
	// }

	// data, err := ioutil.ReadFile(cp)
	// if err == nil {
	// 	json.Unmarshal(data, globalConfig)
	// 	log.Printf("%v\n", globalConfig)
	// }

	pm := proc.NewProcessMonitor()

	// update config in background
	go func() {
		for {
			// get config from remote or local config file
			// cfg := &Config{}
			time.Sleep(3 * time.Second)
		}
	}()

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
	http.ListenAndServe(":10001", nil)
}
