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
	"os"
	"path"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/subprocess"
	"go.uber.org/zap"
)

const findExecutableErrorMsg = "unable to find collector executable path.  Be sure to run `make otelcol`"

type CollectorProcess struct {
	Path             string
	ConfigPath       string
	Logger           *zap.Logger
	Process          *subprocess.Subprocess
	subprocessConfig *subprocess.Config
}

// To be used as a builder whose Build() method provides the actual instance capable of launching the process.
func NewCollectorProcess() CollectorProcess {
	return CollectorProcess{}
}

// Walks up to five parent directories looking for bin/otelcol
func FindCollectorPath() (string, error) {
	var collectorPath string
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := path.Dir(wd)
	for i := 0; i < 5; i++ {
		var up string
		for j := 0; j < i; j++ {
			up = path.Join(up, "..")
		}
		attemptedPath := path.Join(dir, up, "bin", "otelcol")

		info, err := os.Stat(attemptedPath)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("%s: %w", findExecutableErrorMsg, err)
		}
		if info != nil {
			collectorPath = attemptedPath
			break
		}
	}
	if collectorPath == "" || err != nil {
		if err != nil {
			err = fmt.Errorf("%s: %w", findExecutableErrorMsg, err)
		} else {
			err = fmt.Errorf(findExecutableErrorMsg)
		}
	}
	return collectorPath, err
}

func (collector CollectorProcess) WithExecutable(path string) CollectorProcess {
	collector.Path = path
	return collector
}

func (collector CollectorProcess) WithConfig(path string) CollectorProcess {
	collector.ConfigPath = path
	return collector
}

func (collector CollectorProcess) WithLogger(logger *zap.Logger) CollectorProcess {
	collector.Logger = logger
	return collector
}

func (collector CollectorProcess) Build() (*CollectorProcess, error) {
	if collector.ConfigPath == "" {
		return nil, fmt.Errorf("you must specify a Config path for your CollectorProcess before starting")
	}
	if collector.Path == "" {
		collectorPath, err := FindCollectorPath()
		if err != nil {
			return nil, err
		}
		collector.Path = collectorPath
	}
	if collector.Logger == nil {
		collector.Logger = zap.NewNop()
	}
	collector.subprocessConfig = &subprocess.Config{
		ExecutablePath: collector.Path,
		Args:           []string{"--log-level", "debug", "--config", collector.ConfigPath},
	}
	collector.Process = subprocess.NewSubprocess(collector.subprocessConfig, collector.Logger)
	return &collector, nil
}

func (collector *CollectorProcess) Start(ctx context.Context) error {
	if collector.Process == nil {
		return fmt.Errorf("cannot Start a CollectorProcess that hasn't been successfully built")
	}
	go func() {
		// required to avoid needlessly filling buffer
		for _ = range collector.Process.Stdout {
		}
	}()

	return collector.Process.Start(ctx)
}
