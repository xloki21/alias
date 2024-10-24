package base

import "github.com/xloki21/alias/internal/repository"

type Credentials struct {
	AuthSource string
	User       string
	Password   string
}

type MongoDBStorageConfig struct {
	URI         string `mapstructure:"uri"`
	Credentials Credentials
	Database    string `mapstructure:"database"`
}

type StorageConfig struct {
	Type    repository.Type       `yaml:"type"` // mongodb/postgres/inmemory
	MongoDB *MongoDBStorageConfig `mapstructure:"config" yaml:"config"`
}
