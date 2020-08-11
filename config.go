package main

import (
	"github.com/andersfylling/disgord"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfg config

// misc configuration that is not secret
type config struct {
	Prefix        string
	Guild         disgord.Snowflake
	HelpChannel   disgord.Snowflake `mapstructure:"help-channel"`
	LobbyChannel  disgord.Snowflake `mapstructure:"lobby-channel"`
	StudentRole   disgord.Snowflake `mapstructure:"student-role"`
	AssistantRole disgord.Snowflake `mapstructure:"assistant-role"`
	GitHubOrg     string            `mapstructure:"gh-org"`
}

func initConfig() (err error) {
	// command line
	pflag.String("token", "", "Discord Bot Token")
	pflag.String("gh-token", "", "GitHub token with access to the course's organization")
	pflag.String("gh-org", "", "GitHub organization name")
	pflag.String("db-path", "file::memory:?cache=shared", "Path to database file (defaults to in-memory)")
	pflag.String("prefix", "!", "Prefix for all commands")
	pflag.Uint64("guild", 0, "Guild ID")
	pflag.Uint64("help-channel", 0, "Text channel to serve")
	pflag.Uint64("lobby-channel", 0, "Voice channel to direct users to")
	pflag.Uint64("student-role", 0, "Role ID for students")
	pflag.Uint64("assistant-role", 0, "Role ID for teaching assistants")
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
