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
	"testing"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/core/config"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/genericjmx"
	"github.com/signalfx/signalfx-agent/pkg/monitors/collectd/kafka"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestSfxSmartAgentReceiver(t *testing.T) {
	type args struct {
		config       Config
		nextConsumer consumer.MetricsConsumer
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "default_endpoint",
			args: args{
				config: Config{
					ReceiverSettings: configmodels.ReceiverSettings{
						TypeVal: typeStr,
						NameVal: typeStr + "/kafka",
					},
					monitorConfig: &kafka.Config{
						Config: genericjmx.Config{
							MonitorConfig: config.MonitorConfig{
								Type:            "collectd/kafka",
								IntervalSeconds: 5,
							},
							Host:       "localhost",
							Port:       7199,
							ServiceURL: "service:jmx:rmi:///jndi/rmi://{{.Host}}:{{.Port}}/jmxrmi",
						},
						ClusterName: "somecluster",
					},
				},
				nextConsumer: new(consumertest.MetricsSink),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// consumer := tt.args.nextConsumer
			consumer := new(consumertest.MetricsSink)
			got := NewReceiver(zap.NewNop(), tt.args.config, consumer)

			err := got.Start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err)

			assert.Eventuallyf(t, func() bool {
				return len(consumer.AllMetrics()) > 0
			}, 30*time.Second, 1*time.Second, "failed to receive any metrics")

			metrics := consumer.AllMetrics()
			for _, m := range metrics {
				fmt.Printf("%#v\n", m)
				mCount, dCount := m.MetricAndDataPointCount()
				fmt.Printf("%v - %v\n", mCount, dCount)
				rms := m.ResourceMetrics()
				fmt.Printf("resourceMetrics: %v\n", rms)
				for i := 0; i < rms.Len(); i++ {
					rm := rms.At(i)
					resource := rm.Resource()
					attrs := resource.Attributes()
					fmt.Printf("metric: %#v , resource: %#v , attrs: %#v\n", rm, resource, attrs)
					attrs.ForEach(func(k string, v pdata.AttributeValue) {
						fmt.Printf("k: %v - v: %v\n", k, v)
					})

					ilm := rm.InstrumentationLibraryMetrics()
					fmt.Printf("ilm: %v\n", ilm)
					for j := 0; j < ilm.Len(); j++ {
						lm := ilm.At(j)
						il := lm.InstrumentationLibrary()
						mets := lm.Metrics()
						fmt.Printf("lm: %#v, il: %#v, mets: %#v\n", lm, il, mets)
						for k := 0; k < mets.Len(); k++ {
							mt := mets.At(k)
							fmt.Printf("mt: %#v\n", mt)
							name := mt.Name()
							dtype := mt.DataType().String()
							fmt.Printf("name: %#v\n", name)
							fmt.Printf("dtype: %#v\n", dtype)
							switch dtype {
							case "IntGauge":
								ig := mt.IntGauge()
								fmt.Printf("ig: %#v\n", ig)
								for l := 0; l < ig.DataPoints().Len(); l++ {
									igdp := ig.DataPoints().At(l)
									labels := igdp.LabelsMap()
									value := igdp.Value()
									fmt.Printf("igbg: labels: %#v, value: %v\n", labels, value)
									labels.ForEach(func(k string, v string) {
										fmt.Printf("k: %v - v: %v\n", k, v)
									})
								}
							case "IntSum":
								is := mt.IntSum()
								fmt.Printf("is: %#v\n", is)
								for l := 0; l < is.DataPoints().Len(); l++ {
									isdp := is.DataPoints().At(l)
									labels := isdp.LabelsMap()
									value := isdp.Value()
									fmt.Printf("isbg: labels: %#v, value: %v\n", labels, value)
									labels.ForEach(func(k string, v string) {
										fmt.Printf("k: %v - v: %v\n", k, v)
									})
								}
							case "DoubleGauge":
								dg := mt.DoubleGauge()
								fmt.Printf("dg: %#v\n", dg)
								for l := 0; l < dg.DataPoints().Len(); l++ {
									dgdp := dg.DataPoints().At(l)
									labels := dgdp.LabelsMap()
									value := dgdp.Value()
									fmt.Printf("dgbg: labels: %#v, value: %v\n", labels, value)
									labels.ForEach(func(k string, v string) {
										fmt.Printf("k: %v - v: %v\n", k, v)
									})
								}
							case "DoubleSum":
								ds := mt.DoubleSum()
								fmt.Printf("ds: %#v\n", ds)
								for l := 0; l < ds.DataPoints().Len(); l++ {
									dsdp := ds.DataPoints().At(l)
									labels := dsdp.LabelsMap()
									value := dsdp.Value()
									fmt.Printf("dsbg: labels: %#v, value: %v\n", labels, value)
									labels.ForEach(func(k string, v string) {
										fmt.Printf("k: %v - v: %v\n", k, v)
									})
								}
							default:
								panic(fmt.Errorf("unexpected type: %#v", dtype))
							}
						}
					}
				}
			}
		})
	}
}
