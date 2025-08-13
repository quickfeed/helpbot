package helpbot

import (
	"context"
	"crypto/tls"
	"net/http"

	"connectrpc.com/connect"
	"github.com/quickfeed/quickfeed/qf/qfconnect"
)

type QuickFeed struct {
	qf qfconnect.QuickFeedServiceClient
}

func NewQuickFeed(authToken string) (*QuickFeed, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	qf := qfconnect.NewQuickFeedServiceClient(&client, "https://uis.itest.run", connect.WithInterceptors(tokenAuthClientInterceptor(authToken)))
	return &QuickFeed{
		qf: qf,
	}, nil
}

// NewTokenAuthClientInterceptor returns a client interceptor that will add the given token in the Authorization header.
func tokenAuthClientInterceptor(token string) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Spec().IsClient {
				// Send a token with client requests.
				req.Header().Set("Authorization", token)
			}
			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
