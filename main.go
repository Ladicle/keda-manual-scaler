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

	evCh := make(chan event)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return startKEDAExternalPushScaler(ctx, c, evCh)
	})
	eg.Go(func() error {
		return startAPIServer(ctx, c.HttpAPIPort, evCh)
	})
	return eg.Wait()
}

// startKEDAExternalPushScaler starts a gRPC server that implements the KEDA ExternalScaler interface.
func startKEDAExternalPushScaler(ctx context.Context, c *config, evCh <-chan event) error {
	log := logr.FromContextOrDiscard(ctx)

	scaler := NewScaler(ctx, c)
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, scaler)
	reflection.Register(grpcServer)
	healthCheck := health.NewServer()
	healthCheck.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthCheck)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-evCh:
				scaler.updateStatus(ev)
			}
		}
	}()

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", c.GrpcScalerPort))
	if err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		log.Info("Starting KEDA external push scaler...", "port", c.GrpcScalerPort)
		errCh <- grpcServer.Serve(l)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		grpcServer.GracefulStop()
		return nil
	}
}

// startAPIServer starts a simple HTTP API server that accepts requests to change the event of the scaler.
func startAPIServer(ctx context.Context, port int, evCh chan<- event) error {
	log := logr.FromContextOrDiscard(ctx)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		activeStr := query.Get("active")
		active, err := strconv.ParseBool(activeStr)
		if err != nil {
			log.Error(err, "Invalid active parameter")
			http.Error(w, "Invalid active parameter", http.StatusBadRequest)
			return
		}
		valueStr := query.Get("value")
		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			log.Error(err, "Invalid value parameter")
			http.Error(w, "Invalid value parameter", http.StatusBadRequest)
		}
		evCh <- event{
			objectName:  query.Get("name"),
			active:      active,
			metricValue: value,
		}
		w.WriteHeader(http.StatusOK)
	})

	errCh := make(chan error)
	go func() {
		log.Info("Starting HTTP API server...", "port", port)
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}
