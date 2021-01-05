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
	"path"
	"testing"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/core/config"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/genericjmx"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/kafka"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/memcached"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/redis"
	"github.com/signalfx/signalfx-agent/pkg/monitors/haproxy"
	"github.com/signalfx/signalfx-agent/pkg/monitors/telegraf/monitors/ntpq"
	"github.com/signalfx/signalfx-agent/pkg/utils/timeutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 5)

	redisCfg := cfg.Receivers["smartagent/redis"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr + "/redis",
		},
		monitorConfig: &redis.Config{
			MonitorConfig: config.MonitorConfig{
				Type:                "collectd/redis",
				IntervalSeconds:     234,
				DatapointsToExclude: []config.MetricFilter{},
			},
			Host: "localhost",
			Port: 6379,
		},
	}, redisCfg)

	kafkaCfg := cfg.Receivers["smartagent/kafka"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr + "/kafka",
		},
		monitorConfig: &kafka.Config{
			Config: genericjmx.Config{
				MonitorConfig: config.MonitorConfig{
					Type:                "collectd/kafka",
					IntervalSeconds:     345,
					DatapointsToExclude: []config.MetricFilter{},
				},
				Host:       "localhost",
				Port:       7199,
				ServiceURL: "service:jmx:rmi:///jndi/rmi://{{.Host}}:{{.Port}}/jmxrmi",
			},
			ClusterName: "somecluster",
		},
	}, kafkaCfg)

	tr := true
	procstatCfg := cfg.Receivers["smartagent/ntpq"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr + "/ntpq",
		},
		monitorConfig: &ntpq.Config{
			MonitorConfig: config.MonitorConfig{
				Type:                "telegraf/ntpq",
				IntervalSeconds:     567,
				DatapointsToExclude: []config.MetricFilter{},
			},
			DNSLookup: &tr,
		},
	}, procstatCfg)

	memcachedCfg := cfg.Receivers["smartagent/memcached"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr + "/memcached",
		},
		monitorConfig: &memcached.Config{
			MonitorConfig: config.MonitorConfig{
				Type:                "collectd/memcached",
				IntervalSeconds:     456,
				DatapointsToExclude: []config.MetricFilter{},
			},
			Host: "localhost",
			Port: 5309,
		},
	}, memcachedCfg)

	haproxyCfg := cfg.Receivers["smartagent/haproxy"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr + "/haproxy",
		},
		monitorConfig: &haproxy.Config{
			MonitorConfig: config.MonitorConfig{
				Type:                "haproxy",
				IntervalSeconds:     123,
				DatapointsToExclude: []config.MetricFilter{},
			},
			Username:  "SomeUser",
			Password:  "secret",
			Path:      "stats?stats;csv",
			SSLVerify: true,
			Timeout:   timeutil.Duration(5 * time.Second),
		},
	}, haproxyCfg)
}
