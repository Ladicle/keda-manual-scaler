package main

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
)

type scaler struct {
	externalscaler.UnimplementedExternalScalerServer

	eventCh chan bool

	logger logr.Logger
}

func (e *scaler) IsActive(_ context.Context, ref *externalscaler.ScaledObjectRef) (*externalscaler.IsActiveResponse, error) {
	e.logger.Info("IsActive called", "ref", ref)
	return &externalscaler.IsActiveResponse{}, nil
}
func (e *scaler) StreamIsActive(ref *externalscaler.ScaledObjectRef, epsServer externalscaler.ExternalScaler_StreamIsActiveServer) error {
	log := e.logger.WithValues("ref", ref)
	log.Info("StreamIsActive called")
	for {
		select {
		case <-epsServer.Context().Done():
			// the call completed? exit
			log.Info("StreamIsActive call completed", "ref", ref)
			return nil
		case active := <-e.eventCh:
			log.Info("sending IsActive response", "active", active)
			err := epsServer.Send(&externalscaler.IsActiveResponse{
				Result: active,
			})
			if err != nil {
				log.Error(err, "failed to send IsActive response", "ref", ref)
			}
		}
	}
}

func (e *scaler) GetMetricSpec(_ context.Context, ref *externalscaler.ScaledObjectRef) (*externalscaler.GetMetricSpecResponse, error) {
	e.logger.Info("GetMetricSpec called", "ref", ref)
	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{MetricName: "dummy", TargetSize: 1},
		},
	}, nil
}

func (e *scaler) GetMetrics(_ context.Context, ref *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	e.logger.Info("GetMetrics called", "ref", ref)
	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{MetricName: "dummy", MetricValue: 0},
		},
	}, nil
}
