package config

import (
	"fmt"
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type (
	// Config is the container of the config settings
	Config struct {
		App          `yaml:"app"`
		GRPC         `yaml:"grpc"`
		FilesStorage `yaml:"files_storage"`
		Log          `yaml:"logger"`
		Cache        `yaml:"cache"`
		FSAPI        `yaml:"fsapi"`
	}

	// App is the structure for the application settings
	App struct {
		Name    string `env-required:"true" yaml:"name"    env:"APP_NAME"`
		Version string `env-required:"true" yaml:"version" env:"APP_VERSION"`
	}

	// GRPC is the structure for the gRPC API related settings
	GRPC struct {
		Port string `yaml:"port" env:"GRPC_PORT"`
	}

	// FilesStorage is the structure for local files management settings
	FilesStorage struct {
		Location string `yaml:"location" env:"FILES_LOCATION"`
	}

	// Log is the structure for the log management settings
	Log struct {
		Level string `env-required:"true" yaml:"log_level" env:"LOG_LEVEL"`
	}

	// Cache is the structure for the cache settings
	Cache struct {
		Endpoint string `yaml:"endpoint" env:"REDIS_ENDPOINT"`
		User     string `yaml:"user" env:"REDIS_USER"`
		Password string `yaml:"password" env:"REDIS_PASSWORD"`
	}

	// FSAPI is for defining settings about a remote FileStorage service
	FSAPI struct {
		Endpoint string `yaml:"endpoint" env:"FSAPI_ENDPOINT"`
	}
)

// NewConfig returns app config.
func NewConfig(configFileName string) (*Config, error) {
	cfg := &Config{}

	configFilePath := "./config/" + configFileName + ".yml"
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
