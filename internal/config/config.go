package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Business BusinessConfig `mapstructure:"business"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers []string         `mapstructure:"brokers"`
	Topic   KafkaTopicConfig `mapstructure:"topic"`
}

type KafkaTopicConfig struct {
	PayResult    string `mapstructure:"pay_result"`
	OrderTimeout string `mapstructure:"order_timeout"`
}

type BusinessConfig struct {
	OrderTimeoutMinutes int `mapstructure:"order_timeout_minutes"`
	MaxRetryCount       int `mapstructure:"max_retry_count"`
}

var GlobalConfig *Config

// LoadConfig 加载配置文件
func LoadConfig(configPath string) *Config {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	GlobalConfig = config
	return config
}
