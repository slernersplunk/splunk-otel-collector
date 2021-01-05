// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smartagentreceiver

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/signalfx/signalfx-agent/pkg/core/common/constants"
	"github.com/signalfx/signalfx-agent/pkg/core/config"
	"github.com/signalfx/signalfx-agent/pkg/core/meta"
	"github.com/signalfx/signalfx-agent/pkg/monitors"
	"github.com/signalfx/signalfx-agent/pkg/monitors/types"
	"github.com/signalfx/signalfx-agent/pkg/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type Receiver struct {
	logger       *zap.Logger
	config       *Config
	monitor      *interface{}
	output       *Output
	nextConsumer consumer.MetricsConsumer

	startOnce sync.Once
	stopOnce  sync.Once
}

var _ component.MetricsReceiver = (*Receiver)(nil)

var (
	globalLock     sync.Mutex
	monitorManager *monitors.MonitorManager
)

func NewReceiver(logger *zap.Logger, config Config, nextConsumer consumer.MetricsConsumer) *Receiver {
	return &Receiver{
		logger:       logger,
		config:       &config,
		nextConsumer: nextConsumer,
	}
}

func (r *Receiver) Start(_ context.Context, host component.Host) error {
	monitorConfig := r.config.monitorConfig
	monitorConfigCore := monitorConfig.(config.MonitorCustomConfig).MonitorConfigCore()
	monitorType := monitorConfigCore.Type
	monitorName := strings.ReplaceAll(r.config.Name(), "/", "")
	monitorConfigCore.MonitorID = types.MonitorID(monitorName)
	if err := r.config.validate(); err != nil {
		return fmt.Errorf("config validation failed for %q: %w", r.config.Name(), err)
	}

	monitorFactory := monitors.MonitorFactories[monitorType]

	r.startManager()
	monitor := monitorFactory()
	r.monitor = &monitor

	output := &Output{nextConsumer: r.nextConsumer, logger: *r.logger}
	r.output = output

	// Taken from signalfx-agent activemonitor.  Should be exported in that lib in future.
	outputValue := utils.FindFieldWithEmbeddedStructs(monitor, "Output",
		reflect.TypeOf((*types.Output)(nil)).Elem())
	if !outputValue.IsValid() {
		outputValue = utils.FindFieldWithEmbeddedStructs(monitor, "Output",
			reflect.TypeOf((*types.FilteringOutput)(nil)).Elem())
		if !outputValue.IsValid() {
			return fmt.Errorf("invalid monitor instance: %#v", monitor)
		}
	}
	outputValue.Set(reflect.ValueOf(output))

	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = config.CallConfigure(monitor, monitorConfig)
	})
	return err
}

func (r *Receiver) Shutdown(context.Context) error {
	err := componenterror.ErrAlreadyStopped
	if r.monitor == nil {
		err = fmt.Errorf("smartagentreceiver's Shutdown() called before Start()")
	} else if shutdownable, ok := (*r.monitor).(monitors.Shutdownable); !ok {
		err = fmt.Errorf("invalid monitor state at Shutdown(): %#v", r.monitor)
	} else {
		r.stopOnce.Do(func() {
			shutdownable.Shutdown()
			err = nil
		})
	}
	return err
}

func (r *Receiver) startManager() {
	globalLock.Lock()
	defer globalLock.Unlock()

	if monitorManager == nil {
		agentMeta := &meta.AgentMeta{
			InternalStatusHost: "0.0.0.",
			InternalStatusPort: 12345,
		}
		monitorManager = monitors.NewMonitorManager(agentMeta)
	}

	collectdConfig := &config.CollectdConfig{
		DisableCollectd:      false,
		Timeout:              40,
		ReadThreads:          5,
		WriteThreads:         2,
		WriteQueueLimitHigh:  500000,
		WriteQueueLimitLow:   400000,
		LogLevel:             "notice",
		IntervalSeconds:      10,
		WriteServerIPAddr:    "127.9.8.7",
		WriteServerPort:      0,
		ConfigDir:            "/etc/signalfx",
		BundleDir:            os.Getenv(constants.BundleDirEnvVar),
		HasGenericJMXMonitor: true,
		// InstanceName string `yaml:"-"`
		// WriteServerQuery string `yaml:"-"`
	}
	var monitorConfigs []config.MonitorConfig
	monitorManager.Configure(monitorConfigs, collectdConfig, 10)
	r.logger.Debug("Configured monitorManager.")
}
