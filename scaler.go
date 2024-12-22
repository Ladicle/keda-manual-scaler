package main

import (
	"context"
	"errors"
	"sync"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
)

func NewScaler(ctx context.Context, c *config) *scaler {
	return &scaler{
		metricName:  c.DefaultConfig.metricName,
		active:      c.DefaultConfig.active,
		targetSize:  c.DefaultConfig.targetSize,
		metricValue: c.DefaultConfig.metricValue,
		logger:      logr.FromContextOrDiscard(ctx).WithName("scaler"),
	}
}

type scaler struct {
	externalscaler.UnimplementedExternalScalerServer

	metricName   string
	active       bool
	targetSize   int64
	metricValue  int64
	objectStatus map[string]status
	mu           sync.RWMutex

	logger logr.Logger
}

type status struct {
	metricValue int64
	active      bool
	activateCh  chan bool
}

type event struct {
	objectName  string
	active      bool
	metricValue int64
}

func (e *scaler) updateStatus(ev event) {
	log := e.logger.WithValues("objectName", ev.objectName)

	e.mu.Lock()
	defer e.mu.Unlock()

	if ev.objectName == "" {
		e.metricValue = ev.metricValue
		e.active = ev.active
		return
	}
	obj, ok := e.objectStatus[ev.objectName]
	if !ok {
		log.Error(errors.New("failed to update object status"),
			"Object is not registered in KEDA yet")
		return
	}
	obj.metricValue = ev.metricValue
	obj.active = ev.active
	select {
	case obj.activateCh <- ev.active:
	default:
		log.V(1).Info("skipping update of object status",
			"reason", "channel capacity is full")
	}
	e.objectStatus[ev.objectName] = obj
}

func (e *scaler) getStatus(objectName string) (bool, int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	obj, ok := e.objectStatus[objectName]
	if !ok {
		return e.active, e.metricValue
	}
	return obj.active, obj.metricValue
}

func (e *scaler) IsActive(_ context.Context, ref *externalscaler.ScaledObjectRef) (*externalscaler.IsActiveResponse, error) {
	e.logger.V(1).Info("IsActive called", "ref", ref)
	active, _ := e.getStatus(ref.Name)
	return &externalscaler.IsActiveResponse{Result: active}, nil
}

func (e *scaler) StreamIsActive(ref *externalscaler.ScaledObjectRef, epsServer externalscaler.ExternalScaler_StreamIsActiveServer) error {
	log := e.logger.WithValues("ref", ref)
	log.Info("StreamIsActive called")

	status := status{
		metricValue: e.metricValue,
		active:      e.active,
		activateCh:  make(chan bool, 1),
	}
	e.mu.Lock()
	e.objectStatus[ref.Name] = status
	e.mu.Unlock()

	for {
		select {
		case <-epsServer.Context().Done():
			log.Info("StreamIsActive call completed", "ref", ref)
			e.mu.Lock()
			delete(e.objectStatus, ref.Name)
			e.mu.Unlock()
			return nil
		case active := <-status.activateCh:
			log.V(1).Info("sending IsActive response", "active", active)
			if err := epsServer.Send(&externalscaler.IsActiveResponse{
				Result: active,
			}); err != nil {
				log.Error(err, "failed to send IsActive response", "ref", ref)
			}
		}
	}
}

func (e *scaler) GetMetricSpec(_ context.Context, ref *externalscaler.ScaledObjectRef) (*externalscaler.GetMetricSpecResponse, error) {
	e.logger.V(1).Info("GetMetricSpec called", "ref", ref)
	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{MetricName: e.metricName, TargetSize: e.targetSize},
		},
	}, nil
}

func (e *scaler) GetMetrics(_ context.Context, ref *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	e.logger.V(1).Info("GetMetrics called", "ref", ref)
	_, val := e.getStatus(ref.ScaledObjectRef.Name)
	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{MetricName: e.metricName, MetricValue: val},
		},
	}, nil
}
