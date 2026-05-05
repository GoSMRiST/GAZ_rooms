package middleware

import (
	"context"
	"net/http"

	proto "github.com/GoSMRiST/protosGaz/gen/go/token"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type TokenValidator struct {
	client proto.TokenClient
	conn   *grpc.ClientConn
}

func NewTokenValidator(authGrpcAddr string) (*TokenValidator, error) {
	conn, err := grpc.NewClient(authGrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &TokenValidator{
		client: proto.NewTokenClient(conn),
		conn:   conn,
	}, nil
}

func (tv *TokenValidator) Close() error {
	return tv.conn.Close()
}

func (tv *TokenValidator) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
			return
		}

		tokenStr := authHeader[len(prefix):]

		resp, err := tv.client.ValidateToken(c.Request.Context(), &proto.ValidateTokenRequest{
			Token: tokenStr,
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), UserIDKey, int(resp.UserId))
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func GetUserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(UserIDKey).(int)

	return id, ok
}
