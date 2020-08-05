package main

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type config struct {
	Prefix        string
	Guild         string
	HelpChannel   string `mapstructure:"help-channel"`
	LobbyChannel  string `mapstructure:"lobby-channel"`
	StudentRole   string `mapstructure:"student-role"`
	AssistantRole string `mapstructure:"assistant-role"`
}

func initConfig() (err error) {
	// command line
	pflag.String("token", "", "Bot Token")
	pflag.String("db-path", "file::memory:?cache=shared", "Path to database file (defaults to in-memory)")
	pflag.String("prefix", "!", "Prefix for all commands")
	pflag.String("guild", "", "Guild ID")
	pflag.String("help-channel", "", "Text channel to serve")
	pflag.String("lobby-channel", "", "Voice channel to direct users to")
	pflag.String("student-role", "", "Role ID for students")
	pflag.String("assistant-role", "", "Role ID for teaching assistants")
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

	err = viper.Unmarshal(&cfg)
	return
}
