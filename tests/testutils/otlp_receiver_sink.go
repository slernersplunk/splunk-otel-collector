// Copyright 2021 Splunk, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/zap"
)

// To be used as a builder whose Build() method provides the actual instance capable of starting the OTLP Receiver
// providing received metrics to test cases.
type OTLPReceiverSink struct {
	Sink     *consumertest.MetricsSink
	Receiver *component.MetricsReceiver
	Endpoint string
	Logger   *zap.Logger
	Host     component.Host
}

func NewOTLPReceiverSink() *OTLPReceiverSink {
	return &OTLPReceiverSink{}
}

func (otlp OTLPReceiverSink) WithLogger(logger *zap.Logger) OTLPReceiverSink {
	otlp.Logger = logger
	return otlp
}

func (otlp OTLPReceiverSink) WithEndpoint(endpoint string) OTLPReceiverSink {
	otlp.Endpoint = endpoint
	return otlp
}

func (otlp OTLPReceiverSink) WithHost(host component.Host) OTLPReceiverSink {
	otlp.Host = host
	return otlp
}

func (otlp OTLPReceiverSink) Build() (*OTLPReceiverSink, error) {
	if otlp.Endpoint == "" {
		otlp.Endpoint = "localhost:4317"
	}
	if otlp.Logger == nil {
		otlp.Logger = zap.NewNop()
	}
	if otlp.Host == nil {
		otlp.Host = componenttest.NewNopHost()
	}

	otlp.Sink = new(consumertest.MetricsSink)

	otlpFactory := otlpreceiver.NewFactory()
	otlpConfig := otlpFactory.CreateDefaultConfig().(*otlpreceiver.Config)
	otlpConfig.GRPC.NetAddr = confignet.NetAddr{Endpoint: otlp.Endpoint, Transport: "tcp"}
	otlpConfig.HTTP = nil

	params := component.ReceiverCreateParams{Logger: otlp.Logger}
	receiver, err := otlpFactory.CreateMetricsReceiver(context.Background(), params, otlpConfig, otlp.Sink)
	if err != nil {
		return nil, err
	}
	otlp.Receiver = &receiver
	return &otlp, nil
}

func (otlp *OTLPReceiverSink) assertBuilt(operation string) error {
	if otlp.Receiver == nil || otlp.Sink == nil {
		return fmt.Errorf("cannot %s() an OTLPReceiverSink that hasn't been built", operation)
	}
	return nil
}

func (otlp *OTLPReceiverSink) Start() error {
	if err := otlp.assertBuilt("Start"); err != nil {
		return err
	}
	return (*otlp.Receiver).Start(context.Background(), otlp.Host)
}

func (otlp *OTLPReceiverSink) Shutdown() error {
	if err := otlp.assertBuilt("Shutdown"); err != nil {
		return err
	}
	return (*otlp.Receiver).Shutdown(context.Background())
}
func (otlp *OTLPReceiverSink) AllMetrics() []pdata.Metrics {
	if err := otlp.assertBuilt("AllMetrics"); err != nil {
		return nil
	}
	return otlp.Sink.AllMetrics()
}

func (otlp *OTLPReceiverSink) MetricsCount() int {
	if err := otlp.assertBuilt("MetricsCount"); err != nil {
		return 0
	}
	return otlp.Sink.MetricsCount()
}

func (otlp *OTLPReceiverSink) Reset() {
	if err := otlp.assertBuilt("Reset"); err == nil {
		otlp.Sink.Reset()
	}
}

func (otlp *OTLPReceiverSink) AssertAllMetricsReceived(t *testing.T, expectedResourceMetrics ResourceMetrics, waitTime time.Duration) error {
	if len(expectedResourceMetrics.ResourceMetrics) == 0 {
		return fmt.Errorf("empty ResourceMetrics provided")
	}

	receivedMetrics := ResourceMetrics{}

	var err error
	assert.Eventually(t, func() bool {
		if otlp.MetricsCount() == 0 {
			return false
		}
		receivedOTLPMetrics := otlp.AllMetrics()
		otlp.Reset()

		var receivedResourceMetrics ResourceMetrics
		receivedResourceMetrics, err = PDataToResourceMetrics(receivedOTLPMetrics...)
		require.NoError(t, err)
		require.NotNil(t, receivedResourceMetrics)
		receivedMetrics = FlattenResourceMetrics(receivedMetrics, receivedResourceMetrics)

		var containsAll bool
		containsAll, err = receivedMetrics.ContainsAll(expectedResourceMetrics)
		return containsAll
	}, waitTime, 1*time.Second, "Failed to receive expected metrics")
	return err
}
