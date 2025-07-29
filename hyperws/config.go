package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Configuration de l'application
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	Node struct {
		DataPath string `yaml:"data_path"`
	} `yaml:"node"`

	Proxy struct {
		MaxClients        int `yaml:"max_clients"`
		HeartbeatInterval int `yaml:"heartbeat_interval"`
		MessageBufferSize int `yaml:"message_buffer_size"`
	} `yaml:"proxy"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

// LoadConfig charge la configuration depuis un fichier YAML
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Valeurs par défaut
	config.Server.Host = "0.0.0.0"
	config.Server.Port = 8080
	config.Node.DataPath = "/var/lib/docker/volumes/node_hl-data-mainnet/_data"
	config.Proxy.MaxClients = 1000
	config.Proxy.HeartbeatInterval = 30
	config.Proxy.MessageBufferSize = 1024
	config.Logging.Level = "info"
	config.Logging.Format = "text"

	// Si aucun fichier de config n'est spécifié, utiliser les valeurs par défaut
	if configPath == "" {
		return config, nil
	}

	// Lire le fichier de configuration
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'ouverture du fichier de config: %v", err)
	}
	defer file.Close()

	// Décoder le YAML
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage du fichier de config: %v", err)
	}

	return config, nil
}

// GetServerAddress retourne l'adresse complète du serveur
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Validate valide la configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("port serveur invalide: %d", c.Server.Port)
	}

	if c.Node.DataPath == "" {
		return fmt.Errorf("chemin des données du nœud non spécifié")
	}

	if c.Proxy.MaxClients <= 0 {
		return fmt.Errorf("nombre maximum de clients invalide: %d", c.Proxy.MaxClients)
	}

	return nil
} 