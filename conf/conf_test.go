package conf

import (
	"encoding/json"
	"testing"
)

func TestDecodeConfig(t *testing.T) {
	const a = `{
		"command": {
			"includes": [
				"service_box.*"
			]
		},
		"port": {
			"excludes": [
				"27151-27955"
			]
		},
		"x51log": {
			"folder": "/tmp"
		}
	  }`

	var cfg Config
	// var cfg map[string]*interface{}
	err := json.Unmarshal([]byte(a), &cfg)
	if err != nil {
		t.Error(err)
	}

	if (cfg.Command.Includes == nil) || (cfg.Command.Includes[0] != `service_box.*`) {
		t.Errorf("%v\n", cfg)
	}

	if cfg.X51Log.Folder != "/tmp" {
		t.Errorf("%v\n", cfg)
	}
}
