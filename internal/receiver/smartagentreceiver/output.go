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

	"github.com/signalfx/golib/v3/datapoint"
	"github.com/signalfx/golib/v3/event"
	"github.com/signalfx/golib/v3/trace"
	"github.com/signalfx/signalfx-agent/pkg/core/dpfilters"
	"github.com/signalfx/signalfx-agent/pkg/monitors/types"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type Output struct {
	nextConsumer consumer.MetricsConsumer
	logger       zap.Logger
}

var _ types.FilteringOutput = (*Output)(nil)

func (output *Output) AddDatapointExclusionFilter(filter dpfilters.DatapointFilter) {
	output.logger.Debug("AddDatapointExclusionFilter", zap.Any("filter", filter))
}

func (output *Output) EnabledMetrics() []string {
	output.logger.Debug("EnabledMetrics has been called.")
	return []string{}
}

func (output *Output) HasEnabledMetricInGroup(group string) bool {
	output.logger.Debug("HasEnabledMetricInGroup", zap.String("group", group))
	return false
}

func (output *Output) HasAnyExtraMetrics() bool {
	output.logger.Debug("HasAnyExtraMetrics has been called.")
	return false
}

func (output *Output) Copy() types.Output {
	output.logger.Debug("Copy has been called.")
	return output
}

func (output *Output) SendDatapoints(datapoints ...*datapoint.Datapoint) {
	metrics, numDropped := datapointsToMetrics(datapoints)
	output.logger.Debug("SendDatapoints", zap.Any("metrics", metrics), zap.Int("numDropped", numDropped))
	output.nextConsumer.ConsumeMetrics(context.Background(), metrics)
}

func (output *Output) SendEvent(event *event.Event) {
	output.logger.Debug("SendEvent", zap.Any("event", event))
}

func (output *Output) SendSpans(spans ...*trace.Span) {
	output.logger.Debug("SendSpans", zap.Any("Span", spans))
}

func (output *Output) SendDimensionUpdate(dimension *types.Dimension) {
	output.logger.Debug("SendDimensionUpdate", zap.Any("dimension", dimension))
}

func (output *Output) AddExtraDimension(key, value string) {
	output.logger.Debug("AddExtraDimension", zap.String("key", key), zap.String("value", value))
}

func (output *Output) RemoveExtraDimension(key string) {
	output.logger.Debug("RemoveExtraDimension", zap.String("key", key))
}

func (output *Output) AddExtraSpanTag(key, value string) {
	output.logger.Debug("AddExtraSpanTag", zap.String("key", key), zap.String("value", value))
}

func (output *Output) RemoveExtraSpanTag(key string) {
	output.logger.Debug("RemoveExtraSpanTag", zap.String("key", key))
}

func (output *Output) AddDefaultSpanTag(key, value string) {
	output.logger.Debug("AddDefaultSpanTag", zap.String("key", key), zap.String("value", value))
}

func (output *Output) RemoveDefaultSpanTag(key string) {
	output.logger.Debug("RemoveDefaultSpanTag", zap.String("key", key))
}
