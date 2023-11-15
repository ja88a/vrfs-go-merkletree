package config

import (
	"fmt"
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type (
	// Config -.
	Config struct {
		App          `yaml:"app"`
		GRPC         `yaml:"grpc"`
		FilesStorage `yaml:"files_storage"`
		Log          `yaml:"logger"`
		FSAPI				 `yaml:"fsapi"`
	}

	// App -.
	App struct {
		Name    string `env-required:"true" yaml:"name"    env:"APP_NAME"`
		Version string `env-required:"true" yaml:"version" env:"APP_VERSION"`
	}

	// FRPC -.
	GRPC struct {
		Port string `yaml:"port" env:"GRPC_PORT"`
	}

	FilesStorage struct {
		Location string `yaml:"location" env:"FILES_LOCATION"`
	}
	// Log -.
	Log struct {
		Level string `env-required:"true" yaml:"log_level" env:"LOG_LEVEL"`
	}
	
	// FSAPI -.
	FSAPI struct {
		Endpoint string `yaml:"endpoint" env:"FSAPI_ENDPOINT"`
	}
)

// NewConfig returns app config.
func NewConfig(configFileName string) (*Config, error) {
	cfg := &Config{}

	configFilePath := "./config/"+configFileName+".yml"
	err := cleanenv.ReadConfig(configFilePath, cfg)
	if err != nil {
		return nil, fmt.Errorf("Config file error\n%w", err)
	}

	if err := cleanenv.ReadConfig(".env", cfg); err != nil {
		log.Println(err.Error())
		return cfg, nil
	}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}