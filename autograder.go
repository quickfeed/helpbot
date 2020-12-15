package helpbot

import (
	"context"
	"fmt"
	"time"

	agpb "github.com/autograde/quickfeed/ag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Autograder struct {
	cc *grpc.ClientConn
	agpb.AutograderServiceClient
	md metadata.MD
}

func (s *Autograder) Close() {
	s.cc.Close()
}

func NewAutograder(userID int) (*Autograder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, ":9090", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	ag := agpb.NewAutograderServiceClient(cc)
	return &Autograder{
		cc:                      cc,
		AutograderServiceClient: ag,
		md:                      metadata.New(map[string]string{"user": fmt.Sprintf("%d", userID)}),
	}, nil
}
