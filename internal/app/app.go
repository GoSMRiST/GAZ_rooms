package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"rooms/internal/config"
	"rooms/internal/infrastructure/grpcclient"
	"rooms/internal/middleware"
	"rooms/internal/repository"
	"rooms/internal/service"
	"rooms/internal/transport/rest"
)

type App struct {
	log            *slog.Logger
	restServer     *http.Server
	hostAddr       string
	tokenValidator *middleware.TokenValidator
	userClient     *grpcclient.UserGrpcClient
}

func NewApp(log *slog.Logger, cfg *config.Config, db *repository.DataBase) (*App, error) {
	tokenValidator, err := middleware.NewTokenValidator(cfg.AuthGrpcAddr)
	if err != nil {
		return nil, err
	}

	userClient, err := grpcclient.NewUserGrpcClient(cfg.UserGrpcAddr)
	if err != nil {
		_ = tokenValidator.Close()
		return nil, err
	}

	roomService := service.NewRoomService(log, db, userClient)
	roomHandler := rest.NewRoomHandler(log, roomService)

	engine := gin.New()
	engine.Use(gin.Recovery()) // паника не роняет весь сервер
	roomHandler.RegisterRoutes(engine, tokenValidator.AuthMiddleware())

	srv := &http.Server{
		Addr:         cfg.RoomsHostAddress,
		Handler:      engine,
		ReadTimeout:  cfg.ServTimeout,
		WriteTimeout: cfg.ServTimeout,
		IdleTimeout:  cfg.ServTimeout,
	}

	return &App{
		log:            log,
		restServer:     srv,
		hostAddr:       cfg.RoomsHostAddress,
		tokenValidator: tokenValidator,
		userClient:     userClient,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("Rooms REST server started", "addr", a.hostAddr)
	if err := a.restServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	if err := a.restServer.Shutdown(ctx); err != nil {
		a.log.Error("HTTP shutdown error", "error", err)
	}
	if err := a.tokenValidator.Close(); err != nil {
		a.log.Error("TokenValidator gRPC close error", "error", err)
	}
	if err := a.userClient.Close(); err != nil {
		a.log.Error("UserClient gRPC close error", "error", err)
	}
	a.log.Info("Rooms server stopped")
	return nil
}
