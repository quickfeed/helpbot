package main

import (
	"context"

	"github.com/google/go-github/v32/github"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

var gh *github.Client

func initGithub(ctx context.Context) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: viper.GetString("gh-token"),
	})
	tc := oauth2.NewClient(ctx, ts)
	gh = github.NewClient(tc)
}
