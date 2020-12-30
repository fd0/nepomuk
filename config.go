package main

import (
	"fmt"
	"io/ioutil"

	"github.com/fd0/nepomuk/extract"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Correspondents []extract.Correspondent `yaml:"correspondents"`
}

func LoadConfig(filename string) (Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("read config failed: %w", err)
	}

	var cfg Config

	err = yaml.UnmarshalStrict(buf, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("load config failed: %w", err)
	}

	return cfg, nil
}
