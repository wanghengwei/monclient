package conf

import (
	"encoding/json"
	"net/http"
)

type Config struct {
	Command struct {
		Includes []string
		Excludes []string
	}

	Port struct {
		Excludes []string
	}
}

type ConfigLoader interface {
	Load() error
}

type HttpConfigLoader struct {
	httpClient *http.Client
	configUrl  string
	config     *Config
}

func NewHttpConfigLoader(url string, cfg *Config) *HttpConfigLoader {
	return &HttpConfigLoader{
		httpClient: &http.Client{},
		config:     cfg,
		configUrl:  url,
	}
}

func (cl *HttpConfigLoader) Load() error {
	resp, err := cl.httpClient.Get(cl.configUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(cl.config)
	if err == nil {
		return err
	}

	return nil
}

type DefaultConfigLoader struct {
	config *Config
}

func NewDefaultConfigLoader(cfg *Config) *DefaultConfigLoader {
	return &DefaultConfigLoader{cfg}
}

func (cl *DefaultConfigLoader) Load() error {
	if cl.config == nil {
		return nil
	}

	cl.config.Command.Includes = nil
	cl.config.Command.Excludes = []string{`^\[.*\]`}
	cl.config.Port.Excludes = nil

	return nil
}
