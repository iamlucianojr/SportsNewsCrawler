package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/SportsNewsCrawler/cmd/server/factory"
	"github.com/SportsNewsCrawler/internal/app"
	"github.com/SportsNewsCrawler/internal/infra/tracing"
	transport "github.com/SportsNewsCrawler/internal/transport/http"
	"github.com/SportsNewsCrawler/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	fx.New(
		fx.Provide(
			// Config
			config.Load,

			// Infrastructure
			factory.NewMongoClient,
			factory.NewMongoRepository,
			fx.Annotate(
				factory.NewMainKafkaProducer,
				fx.ResultTags(`name:"main_producer"`),
			),
			fx.Annotate(
				factory.NewDLQProducer,
				fx.ResultTags(`name:"dlq_producer"`),
			),
			fx.Annotate(
				factory.NewKafkaConsumer,
				fx.ParamTags(``, `name:"dlq_producer"`, ``),
			),

			// Gateways & Producers
			factory.NewCMSGateway,
			fx.Annotate(
				factory.NewEventProducer,
				fx.ParamTags(`name:"main_producer"`),
			),

			// Providers
			factory.NewProviders,

			// Services
			factory.NewNewsCrawlerService,
			factory.NewCMSSyncService,

			// HTTP Server
			transport.NewHTTPServer,
		),
		fx.Invoke(
			SetupTracer,
			WaitForReady, // Block until dependencies are ready
			RegisterHooks,
			StartServer,
		),
	).Run()
}

// --- Invokers ---

func RegisterHooks(lc fx.Lifecycle, service *app.NewsCrawlerService, syncService *app.CMSSyncService) {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go service.Start(ctx)
			syncService.Start(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
}

func SetupTracer(lc fx.Lifecycle) error {
	ctx := context.Background()
	shutdown, err := tracing.InitTracer(ctx, "news-crawler")
	if err != nil {
		slog.Error("Failed to initialize tracer", "error", err)
		return err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			slog.Info("Shutting down tracer provider")
			return shutdown(ctx)
		},
	})
	return nil
}

// WaitForReady blocks until all dependencies are ready.
func WaitForReady(
	cfg *config.Config,
	mongoClient *mongo.Client,
) error {
	ctx := context.Background()
	waiter := app.NewReadinessWaiter(
		mongoClient,
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
	)
	return waiter.WaitForDependencies(ctx)
}

func StartServer(lc fx.Lifecycle, server *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				slog.Info("Starting health check server", "address", server.Addr)
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("HTTP server failed", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
