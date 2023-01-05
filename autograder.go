package helpbot

import (
	"crypto/tls"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/quickfeed/quickfeed/qf/qfconnect"
	"github.com/quickfeed/quickfeed/web/interceptor"
	"google.golang.org/grpc/metadata"
)

type QuickFeed struct {
	qf qfconnect.QuickFeedServiceClient
	md metadata.MD
}

func NewAutograder(authToken string) (*QuickFeed, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	qf := qfconnect.NewQuickFeedServiceClient(&client, "https://127.0.0.1", connect.WithInterceptors(
		interceptor.NewTokenAuthClientInterceptor(authToken),
	))
	return &QuickFeed{
		qf: qf,
		md: metadata.New(map[string]string{"Authorization": authToken}),
	}, nil
}
