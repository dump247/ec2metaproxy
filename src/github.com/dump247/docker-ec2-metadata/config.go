package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type BindConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (self *BindConfig) Init() {
	if len(self.Host) == 0 {
		self.Host = "127.0.0.1"
	}

	if self.Port == 0 {
		self.Port = 18000
	}
}

func (self BindConfig) Addr() string {
	return fmt.Sprintf("%s:%d", self.Host, self.Port)
}

type MetadataConfig struct {
	Url string `yaml:"url"`
}

func (self *MetadataConfig) Init() {
	if len(self.Url) == 0 {
		self.Url = "http://169.254.169.254"
	}
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func (self *LoggingConfig) Init() {
	if len(self.Level) == 0 {
		self.Level = "info"
	}
}

type PlatformConfig map[string]interface{}

func (self PlatformConfig) GetString(key string, defaultValue string) string {
	if value, found := self[key]; found {
		return value.(string)
	} else {
		return defaultValue
	}
}

type ProxyConfig struct {
	Bind          BindConfig             `yaml:"bind"`
	Metadata      MetadataConfig         `yaml:"metadata"`
	Log           LoggingConfig          `yaml:"log"`
	DefaultRole   string                 `yaml:"default-role"`
	DefaultPolicy map[string]interface{} `yaml:"default-policy"`
	Platform      PlatformConfig         `yaml:"platform"`
}

func (self *ProxyConfig) DefaultPolicyJson() string {
	if len(self.DefaultPolicy) == 0 {
		return ""
	}

	content, _ := json.Marshal(self.DefaultPolicy)
	return string(content)
}

func (self *ProxyConfig) Init() error {
	self.Bind.Init()
	self.Metadata.Init()
	self.Log.Init()

	if len(self.DefaultRole) == 0 {
		return errors.New("default-role is required")
	}

	if len(self.Platform) == 0 {
		return errors.New("platform is required")
	}

	return nil
}

func LoadConfigFile(filename string) (*ProxyConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := ProxyConfig{}
	err = yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		return nil, err
	}

	err = config.Init()
	if err != nil {
		return nil, err
	}

	return &config, nil
}
