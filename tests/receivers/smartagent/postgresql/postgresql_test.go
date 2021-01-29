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

package tests

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/signalfx/splunk-otel-collector/tests/testutils"
)

func postgresContainer(t *testing.T) *testutils.Container {
	postgres := testutils.NewContainer().WithContext(
		path.Join(".", "testdata"),
	).WithEnv(map[string]string{
		"POSTGRES_DB":       "test_db",
		"POSTGRES_USER":     "postgres",
		"POSTGRES_PASSWORD": "postgres",
	}).WithExposedPort("5432:5432").WillWaitForPort("5432").WillWaitForLog(
		"database system is ready to accept connections",
	).BuildRequest()
	err := postgres.Start(context.Background())
	require.NoError(t, err)
	return postgres
}

func splunkOtelCollector(logger *zap.Logger) (*testutils.CollectorProcess, error) {
	return testutils.NewCollectorProcess().WithConfig(path.Join(".", "testdata", "config.yaml")).WithLogger(logger).Build()
}

func getLogsOnFailure(t *testing.T, logObserver *observer.ObservedLogs) {
	if true || !t.Failed() {
		return
	}
	fmt.Printf("Logs: \n")
	for _, statement := range logObserver.All() {
		fmt.Printf("%v\n", statement)
	}
}

func TestPostgresReceiverHappyPath(t *testing.T) {
	expectedResourceMetrics, err := testutils.LoadResourceMetrics(
		path.Join(".", "testdata", "resource_metrics", "scratch.yaml"),
	)
	require.NoError(t, err)
	require.NotNil(t, expectedResourceMetrics)

	postgres := postgresContainer(t)
	defer postgres.Stop(context.Background())

	logCore, logObserver := observer.New(zap.DebugLevel)
	defer getLogsOnFailure(t, logObserver)

	logger := zap.New(logCore)
	otlp, err := testutils.NewOTLPReceiverSink().WithEndpoint("localhost:23456").WithLogger(logger).Build()
	require.NoError(t, err)

	defer func() {
		require.Nil(t, otlp.Shutdown())
	}()
	require.NoError(t, otlp.Start())

	collector, err := splunkOtelCollector(logger)
	require.NoError(t, err)
	require.NotNil(t, collector)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, collector.Start(ctx))

	require.NoError(t, otlp.AssertAllMetricsReceived(t, expectedResourceMetrics, 30*time.Second))
}
