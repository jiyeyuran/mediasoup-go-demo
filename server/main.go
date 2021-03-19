package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/mediasoup-demo/")
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	config := DefaultConfig
	logger := NewLogger("Main")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Errorf("Fatal error read config file: %s\n", err))
		}
	} else {
		logger.Info().Str("file", viper.ConfigFileUsed()).Msg("config file used")

		if err := viper.Unmarshal(&config); err != nil {
			panic(fmt.Errorf("Fatal error unmarshal config file: %s\n", err))
		}
	}

	data, _ := json.MarshalIndent(config, "", "  ")

	fmt.Printf("config:\n%s\n", data)
	// logger.Info().Interface("config", config).Send()

	server := NewServer(config)
	server.Run()
}
