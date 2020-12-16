package main

import (
	"github.com/Raytar/helpbot"
	"github.com/spf13/viper"
)

var cfg []helpbot.Config

func initConfig() (err error) {
	// command line

	// env
	viper.SetEnvPrefix(botName)
	viper.AutomaticEnv()

	// config file
	viper.SetConfigName(cfgFile)
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.UnmarshalKey("instances", &cfg)
	return
}
