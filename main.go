package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	var (
		path     string
		logLevel int
	)
	cmd := &cobra.Command{
		Use: "scaler",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdr.SetVerbosity(logLevel)
			logger := stdr.New(log.Default())
			ctx := logr.NewContext(cmd.Context(), logger)

			c, err := parseConfig(path)
			if err != nil {
				return err
			}

			return run(ctx, c)
		},
	}
	cmd.Flags().StringVar(&path, "config", "", "Path to the config file")
	cmd.Flags().IntVarP(&logLevel, "log-level", "v", 0, "Log level for the manual scaler")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, c *config) error {
	log := logr.FromContextOrDiscard(ctx)
	log.Info("Starting manual scaler", "config", c)

	s := &scaler{eventCh: make(chan bool), logger: log.WithName("scaler")}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return runGrpcServer(ctx, c.GrpcPort, s)
	})
	eg.Go(func() error {
		return httpServer(ctx, c.HttpPort, s)
	})

	return eg.Wait()
}

func runGrpcServer(ctx context.Context, port int, s *scaler) error {
	log := logr.FromContextOrDiscard(ctx)

	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, s)
	reflection.Register(grpcServer)

	healthCheck := health.NewServer()
	healthCheck.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthCheck)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	log.Info("Starting gRPC server...", "port", port)
	return grpcServer.Serve(l)
}

func httpServer(ctx context.Context, port int, s *scaler) error {
	log := logr.FromContextOrDiscard(ctx)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		activeStr := r.URL.Query().Get("active")
		active, err := strconv.ParseBool(activeStr)
		if err != nil {
			log.Error(err, "Invalid active parameter")
			http.Error(w, "Invalid active parameter", http.StatusBadRequest)
			return
		}
		s.eventCh <- active
		w.WriteHeader(http.StatusOK)
	})

	log.Info("Starting HTTP server...", "port", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
