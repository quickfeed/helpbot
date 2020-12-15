package main

import (
	"github.com/Raytar/helpbot"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfg []helpbot.Config

func initConfig() (err error) {
	// command line
	pflag.String("db-path", "file::memory:?cache=shared", "Path to database file (defaults to in-memory)")
	pflag.Parse()

	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return
	}

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
