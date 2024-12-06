package main

import (
	"encoding/json"
	"os"
)

type (
	Config struct {
		Port         uint16        `json:"Port"`
		Account      Account       `json:"Account"`
		ServerConfig *ServerConfig `json:"ServerConfig"`
	}
	Account struct {
		Username string `json:"Username"`
		Password string `json:"Password"`
	}
	ServerConfig struct {
		MaxAuthTries int      `json:"MaxAuthTries"`
		KeyExchanges []string `json:"KeyExchanges"`
		Ciphers      []string `json:"Ciphers"`
		MACs         []string `json:"MACs"`
	}
)

func LoadConfig(file string) (cfg Config, err error) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0600)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	err = dec.Decode(&cfg)
	return cfg, err
}

func (c *Config) SetDefault() {
	if c.Port == 0 {
		c.Port = 4400
	}
	if c.Account.Username == "" {
		c.Account.Username = "tools"
	}
	if c.Account.Password == "" {
		c.Account.Password = "tools"
	}
}
