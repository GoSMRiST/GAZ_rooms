package grpcclient

import (
	"context"
	"rooms/internal/core"

	proto "github.com/GoSMRiST/protosGaz/gen/go/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UserGrpcClient struct {
	client proto.UserClient
	conn   *grpc.ClientConn
}

func NewUserGrpcClient(addr string) (*UserGrpcClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &UserGrpcClient{
		client: proto.NewUserClient(conn),
		conn:   conn,
	}, nil
}

func (c *UserGrpcClient) Close() error {
	return c.conn.Close()
}

func (c *UserGrpcClient) GetUserInfo(ctx context.Context, userID int) (bool, core.Subscription, error) {
	resp, err := c.client.GetUser(ctx, &proto.GetUserRequest{
		UserId: int64(userID),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, core.SubscriptionDefault, nil
		}
		return false, core.SubscriptionDefault, err
	}

	sub := core.Subscription(resp.GetSubscription())
	if _, ok := core.RoomCreationLimits[sub]; !ok {
		sub = core.SubscriptionDefault
	}

	return true, sub, nil
}
