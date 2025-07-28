package config

import (
	"fmt"
	"os"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	
	Hyperliquid struct {
		MainnetURL string `yaml:"mainnet_url"`
		TestnetURL string `yaml:"testnet_url"`
		Network    string `yaml:"network"` // "mainnet" or "testnet"
	} `yaml:"hyperliquid"`
	
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
	
	Proxy struct {
		MaxClients           int  `yaml:"max_clients"`
		EnableHeartbeat      bool `yaml:"enable_heartbeat"`
		HeartbeatInterval    int  `yaml:"heartbeat_interval"`
		ReconnectMaxRetries  int  `yaml:"reconnect_max_retries"`
		ReconnectInterval    int  `yaml:"reconnect_interval"`
		BufferSize           int  `yaml:"buffer_size"`
		EnableLocalNode      bool `yaml:"enable_local_node"`
		LocalNodeDataPath    string `yaml:"local_node_data_path"`
	} `yaml:"proxy"`
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	
	// Default values
	config.Server.Host = "0.0.0.0"
	config.Server.Port = 8080
	config.Hyperliquid.MainnetURL = "wss://api.hyperliquid.xyz/ws"
	config.Hyperliquid.TestnetURL = "wss://api.hyperliquid-testnet.xyz/ws"
	config.Hyperliquid.Network = "mainnet"
	config.Logging.Level = "info"
	config.Logging.Format = "text"
	config.Proxy.MaxClients = 1000
	config.Proxy.EnableHeartbeat = true
	config.Proxy.HeartbeatInterval = 30
	config.Proxy.ReconnectMaxRetries = 5
	config.Proxy.ReconnectInterval = 5
	config.Proxy.BufferSize = 1024
	config.Proxy.EnableLocalNode = false
	config.Proxy.LocalNodeDataPath = "/home/hluser/hl/data"
	
	if configPath == "" {
		return config, nil
	}
	
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer file.Close()
	
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %v", err)
	}
	
	return config, nil
}

func (c *Config) GetHyperliquidURL() string {
	if c.Hyperliquid.Network == "testnet" {
		return c.Hyperliquid.TestnetURL
	}
	return c.Hyperliquid.MainnetURL
}

func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
} 