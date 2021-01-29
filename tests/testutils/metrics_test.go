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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func pdataMetrics() pdata.Metrics {
	metrics := pdata.NewMetrics()
	metrics.ResourceMetrics().Resize(1)
	resourceMetrics := metrics.ResourceMetrics().At(0)
	attrs := resourceMetrics.Resource().Attributes()
	attrs.InsertBool("bool", true)
	attrs.InsertString("string", "a_string")
	attrs.InsertInt("int", 123)
	attrs.InsertDouble("double", 123.456)
	attrs.InsertNull("null")

	ilms := resourceMetrics.InstrumentationLibraryMetrics()
	ilms.Resize(3)

	ilms.At(0).InstrumentationLibrary().SetName("an_instrumentation_library_name")
	ilms.At(0).InstrumentationLibrary().SetVersion("an_instrumentation_library_version")
	ilmOneMetrics := ilms.At(0).Metrics()
	ilmOneMetrics.Resize(2)
	ilmOneMetricOne := ilmOneMetrics.At(0)
	ilmOneMetricOne.SetName("a_metric")
	ilmOneMetricOne.SetDescription("a_metric_description")
	ilmOneMetricOne.SetUnit("a_metric_unit")
	ilmOneMetricOne.SetDataType(pdata.MetricDataTypeIntGauge)
	ilmOneMetricOneDps := ilmOneMetricOne.IntGauge().DataPoints()
	ilmOneMetricOneDps.Resize(2)
	ilmOneMetricOneDps.At(0).SetValue(12345)
	ilmOneMetricOneDps.At(0).LabelsMap().Insert("label_name_1", "label_value_1")
	ilmOneMetricOneDps.At(1).SetValue(23456)
	ilmOneMetricOneDps.At(1).LabelsMap().Insert("label_name_2", "label_value_2")

	ilmOneMetricTwo := ilmOneMetrics.At(1)
	ilmOneMetricTwo.SetName("another_metric")
	ilmOneMetricTwo.SetDescription("another_metric_description")
	ilmOneMetricTwo.SetUnit("another_metric_unit")
	ilmOneMetricTwo.SetDataType(pdata.MetricDataTypeDoubleSum)
	ilmOneMetricTwo.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	ilmOneMetricTwo.DoubleSum().SetIsMonotonic(false)
	ilmOneMetricTwoDps := ilmOneMetricTwo.DoubleSum().DataPoints()
	ilmOneMetricTwoDps.Resize(2)
	ilmOneMetricTwoDps.At(0).SetValue(34567.89)
	ilmOneMetricTwoDps.At(0).LabelsMap().Insert("label_name_3", "label_value_3")
	ilmOneMetricTwoDps.At(1).SetValue(45678.90)
	ilmOneMetricTwoDps.At(1).LabelsMap().Insert("label_name_4", "label_value_4")

	ilms.At(1).InstrumentationLibrary().SetName("an_instrumentation_library_without_version_or_metrics")

	ilmThreeMetrics := ilms.At(2).Metrics()
	ilmThreeMetrics.Resize(2)
	ilmThreeMetricOne := ilmThreeMetrics.At(0)
	ilmThreeMetricOne.SetName("yet_another_metric")
	ilmThreeMetricOne.SetDescription("yet_another_metric_description")
	ilmThreeMetricOne.SetDataType(pdata.MetricDataTypeDoubleGauge)
	ilmThreeMetricOneDps := ilmThreeMetricOne.DoubleGauge().DataPoints()
	ilmThreeMetricOneDps.Resize(2)
	ilmThreeMetricOneDps.At(0).SetValue(12345.678)
	ilmThreeMetricOneDps.At(0).LabelsMap().Insert("label_name_5", "label_value_5")
	ilmThreeMetricOneDps.At(1).SetValue(23456.789)
	ilmThreeMetricOneDps.At(1).LabelsMap().Insert("label_name_6", "label_value_6")

	ilmThreeMetricTwo := ilmThreeMetrics.At(1)
	ilmThreeMetricTwo.SetName("a_yet_another_metric")
	ilmThreeMetricTwo.SetDataType(pdata.MetricDataTypeIntSum)
	ilmThreeMetricTwo.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	ilmThreeMetricTwo.IntSum().SetIsMonotonic(true)
	ilmThreeMetricTwoDps := ilmThreeMetricTwo.IntSum().DataPoints()
	ilmThreeMetricTwoDps.Resize(2)
	ilmThreeMetricTwoDps.At(0).SetValue(345678)
	ilmThreeMetricTwoDps.At(1).SetValue(456789)
	return metrics
}

func TestLoadMetricsHappyPath(t *testing.T) {
	resourceMetrics, err := LoadResourceMetrics(path.Join(".", "testdata", "resourceMetrics.yaml"))
	require.NoError(t, err)
	require.NotNil(t, resourceMetrics)

	assert.Equal(t, 2, len(resourceMetrics.ResourceMetrics))

	firstRM := resourceMetrics.ResourceMetrics[0]
	firstRMAttrs := firstRM.Resource.Attributes
	require.Equal(t, 2, len(firstRMAttrs))
	require.NotNil(t, firstRMAttrs["one_attr"])
	assert.Equal(t, "one_value", firstRMAttrs["one_attr"])
	require.NotNil(t, firstRMAttrs["two_attr"])
	assert.Equal(t, "two_value", firstRMAttrs["two_attr"])

	assert.Equal(t, 2, len(firstRM.ILMs))
	firstRMFirstILM := firstRM.ILMs[0]
	require.NotNil(t, firstRMFirstILM)
	require.NotNil(t, firstRMFirstILM.InstrumentationLibrary)
	assert.Equal(t, "without_metrics", firstRMFirstILM.InstrumentationLibrary.Name)
	assert.Equal(t, "some_version", firstRMFirstILM.InstrumentationLibrary.Version)
	require.Nil(t, firstRMFirstILM.Metrics)

	firstRMSecondILM := firstRM.ILMs[1]
	require.NotNil(t, firstRMSecondILM)
	require.NotNil(t, firstRMSecondILM.InstrumentationLibrary)
	assert.Empty(t, firstRMSecondILM.InstrumentationLibrary.Name)
	assert.Empty(t, firstRMSecondILM.InstrumentationLibrary.Version)
	require.NotNil(t, firstRMSecondILM.Metrics)

	require.Equal(t, 2, len(firstRMSecondILM.Metrics))
	firstRMSecondILMFirstMetric := firstRMSecondILM.Metrics[0]
	require.NotNil(t, firstRMSecondILMFirstMetric)
	assert.Equal(t, "an_int_gauge", firstRMSecondILMFirstMetric.Name)
	assert.Equal(t, "int_gauge", firstRMSecondILMFirstMetric.Type)
	assert.Equal(t, "an_int_gauge_description", firstRMSecondILMFirstMetric.Description)
	assert.Equal(t, "an_int_gauge_unit", firstRMSecondILMFirstMetric.Unit)
	assert.Equal(t, 123, firstRMSecondILMFirstMetric.Value)

	firstRMSecondILMSecondMetric := firstRMSecondILM.Metrics[1]
	require.NotNil(t, firstRMSecondILMSecondMetric)
	assert.Equal(t, "a_double_gauge", firstRMSecondILMSecondMetric.Name)
	assert.Equal(t, "double_gauge", firstRMSecondILMSecondMetric.Type)
	assert.Equal(t, 123.456, firstRMSecondILMSecondMetric.Value)
	assert.Empty(t, firstRMSecondILMSecondMetric.Unit)
	assert.Empty(t, firstRMSecondILMSecondMetric.Description)

	secondRM := resourceMetrics.ResourceMetrics[1]
	require.Zero(t, len(secondRM.Resource.Attributes))

	assert.Equal(t, 1, len(secondRM.ILMs))
	secondRMFirstILM := secondRM.ILMs[0]
	require.NotNil(t, secondRMFirstILM)
	require.NotNil(t, secondRMFirstILM.InstrumentationLibrary)
	assert.Equal(t, "with_metrics", secondRMFirstILM.InstrumentationLibrary.Name)
	assert.Equal(t, "another_version", secondRMFirstILM.InstrumentationLibrary.Version)
	require.NotNil(t, secondRMFirstILM.Metrics)

	require.Equal(t, 2, len(secondRMFirstILM.Metrics))
	secondRMFirstILMFirstMetric := secondRMFirstILM.Metrics[0]
	require.NotNil(t, secondRMFirstILMFirstMetric)
	assert.Equal(t, "another_int_gauge", secondRMFirstILMFirstMetric.Name)
	assert.Equal(t, "int_gauge", secondRMFirstILMFirstMetric.Type)
	assert.Empty(t, secondRMFirstILMFirstMetric.Description)
	assert.Empty(t, secondRMFirstILMFirstMetric.Unit)
	assert.Empty(t, secondRMFirstILMFirstMetric.Value)

	secondRMFirstILMSecondMetric := secondRMFirstILM.Metrics[1]
	require.NotNil(t, secondRMFirstILMSecondMetric)
	assert.Equal(t, "another_double_gauge", secondRMFirstILMSecondMetric.Name)
	assert.Equal(t, "double_gauge", secondRMFirstILMSecondMetric.Type)
	assert.Empty(t, secondRMFirstILMSecondMetric.Value)
	assert.Empty(t, secondRMFirstILMSecondMetric.Unit)
	assert.Empty(t, secondRMFirstILMSecondMetric.Description)
}

func TestPDataToResourceMetrics(t *testing.T) {
	resourceMetrics, err := PDataToResourceMetrics(pdataMetrics())
	assert.NoError(t, err)
	require.NotNil(t, resourceMetrics)

	rms := resourceMetrics.ResourceMetrics
	assert.Len(t, rms, 1)
	rm := rms[0]
	attrs := rm.Resource.Attributes
	assert.True(t, attrs["bool"].(bool))
	assert.Equal(t, "a_string", attrs["string"].(string))
	assert.Equal(t, 123, int(attrs["int"].(int64)))
	assert.Equal(t, 123.456, attrs["double"].(float64))
	assert.Nil(t, attrs["null"])

	ilms := rm.ILMs
	assert.Len(t, ilms, 3)
	assert.Equal(t, "an_instrumentation_library_name", ilms[0].InstrumentationLibrary.Name)
	assert.Equal(t, "an_instrumentation_library_version", ilms[0].InstrumentationLibrary.Version)

	require.Len(t, ilms[0].Metrics, 4)

	ilm0Metric0 := ilms[0].Metrics[0]
	assert.Equal(t, "a_metric", ilm0Metric0.Name)
	assert.Equal(t, "a_metric_description", ilm0Metric0.Description)
	assert.Equal(t, "a_metric_unit", ilm0Metric0.Unit)
	assert.Equal(t, "IntGauge", ilm0Metric0.Type)
	assert.Equal(t, map[string]string{"label_name_1": "label_value_1"}, ilm0Metric0.Labels)
	assert.EqualValues(t, 12345, ilm0Metric0.Value)

	ilm0Metric1 := ilms[0].Metrics[1]
	assert.Equal(t, "a_metric", ilm0Metric1.Name)
	assert.Equal(t, "a_metric_description", ilm0Metric1.Description)
	assert.Equal(t, "a_metric_unit", ilm0Metric1.Unit)
	assert.Equal(t, "IntGauge", ilm0Metric1.Type)
	assert.Equal(t, map[string]string{"label_name_2": "label_value_2"}, ilm0Metric1.Labels)
	assert.EqualValues(t, 23456, ilm0Metric1.Value)

	ilm0Metric2 := ilms[0].Metrics[2]
	assert.Equal(t, "another_metric", ilm0Metric2.Name)
	assert.Equal(t, "another_metric_description", ilm0Metric2.Description)
	assert.Equal(t, "another_metric_unit", ilm0Metric2.Unit)
	assert.Equal(t, "DoubleNonmonotonicDeltaSum", ilm0Metric2.Type)
	assert.Equal(t, map[string]string{"label_name_3": "label_value_3"}, ilm0Metric2.Labels)
	assert.EqualValues(t, 34567.89, ilm0Metric2.Value)

	ilm0Metric3 := ilms[0].Metrics[3]
	assert.Equal(t, "another_metric", ilm0Metric3.Name)
	assert.Equal(t, "another_metric_description", ilm0Metric3.Description)
	assert.Equal(t, "another_metric_unit", ilm0Metric3.Unit)
	assert.Equal(t, "DoubleNonmonotonicDeltaSum", ilm0Metric3.Type)
	assert.Equal(t, map[string]string{"label_name_4": "label_value_4"}, ilm0Metric3.Labels)
	assert.EqualValues(t, 45678.90, ilm0Metric3.Value)

	assert.Equal(t, "an_instrumentation_library_without_version_or_metrics", ilms[1].InstrumentationLibrary.Name)
	assert.Empty(t, ilms[1].InstrumentationLibrary.Version)
	assert.Empty(t, ilms[1].Metrics)

	ilm2Metric0 := ilms[2].Metrics[0]
	assert.Equal(t, "yet_another_metric", ilm2Metric0.Name)
	assert.Equal(t, "yet_another_metric_description", ilm2Metric0.Description)
	assert.Empty(t, ilm2Metric0.Unit)
	assert.Equal(t, "DoubleGauge", ilm2Metric0.Type)
	assert.Equal(t, map[string]string{"label_name_5": "label_value_5"}, ilm2Metric0.Labels)
	assert.EqualValues(t, 12345.678, ilm2Metric0.Value)

	ilm2Metric1 := ilms[2].Metrics[1]
	assert.Equal(t, "yet_another_metric", ilm2Metric1.Name)
	assert.Equal(t, "yet_another_metric_description", ilm2Metric1.Description)
	assert.Empty(t, ilm2Metric1.Unit)
	assert.Equal(t, "DoubleGauge", ilm2Metric1.Type)
	assert.Equal(t, map[string]string{"label_name_6": "label_value_6"}, ilm2Metric1.Labels)
	assert.EqualValues(t, 23456.789, ilm2Metric1.Value)

	ilm2Metric2 := ilms[2].Metrics[2]
	assert.Equal(t, "a_yet_another_metric", ilm2Metric2.Name)
	assert.Empty(t, ilm2Metric2.Description)
	assert.Empty(t, ilm2Metric2.Unit)
	assert.Equal(t, "IntMonotonicCumulativeSum", ilm2Metric2.Type)
	assert.Empty(t, ilm2Metric2.Labels)
	assert.EqualValues(t, 345678, ilm2Metric2.Value)

	ilm2Metric3 := ilms[2].Metrics[3]
	assert.Equal(t, "a_yet_another_metric", ilm2Metric3.Name)
	assert.Empty(t, ilm2Metric3.Description)
	assert.Empty(t, ilm2Metric3.Unit)
	assert.Equal(t, "IntMonotonicCumulativeSum", ilm2Metric3.Type)
	assert.Empty(t, ilm2Metric3.Labels)
	assert.EqualValues(t, 456789, ilm2Metric3.Value)
}

func TestHashFunctions(t *testing.T) {
	resource := Resource{Attributes: map[string]interface{}{
		"one": "1", "two": 2, "three": 3.000, "four": false, "five": nil,
	}}
	for i := 0; i < 100; i++ {
		require.Equal(t, "1f16e8e05a479e68c4fa5950471169e4", resource.Hash())
	}

	il := InstrumentationLibrary{Name: "some instrumentation library", Version: "some instrumentation version"}
	for i := 0; i < 100; i++ {
		require.Equal(t, "74ad4f0b1d06de1b45484cdbcfdd62db", il.Hash())
	}

	metric := Metric{
		Name: "some metric", Description: "some description",
		Unit: "some unit", Labels: map[string]string{
			"labelOne": "1", "labelTwo": "two",
		}, Type: "some metric type", Value: 123.456,
	}
	for i := 0; i < 100; i++ {
		require.Equal(t, "a481752281903890feb2c149573e0eaa", metric.Hash())
	}
}

func TestFlattenResourceMetricsByResourceIdentity(t *testing.T) {
	resource := Resource{Attributes: map[string]interface{}{"attribute_one": nil, "attribute_two": 123.456}}
	resourceMetrics := ResourceMetrics{
		ResourceMetrics: []ResourceMetric{
			ResourceMetric{Resource: resource},
			ResourceMetric{Resource: resource},
			ResourceMetric{Resource: resource},
		},
	}
	expectedResourceMetrics := ResourceMetrics{ResourceMetrics: []ResourceMetric{ResourceMetric{Resource: resource}}}
	require.Equal(t, expectedResourceMetrics, FlattenResourceMetrics(resourceMetrics))
}

func TestFlattenResourceMetricsByInstrumentationLibraryMetricsIdentity(t *testing.T) {
	resource := Resource{Attributes: map[string]interface{}{"attribute_three": true, "attribute_four": 23456}}
	ilm := InstrumentationLibraryMetrics{InstrumentationLibrary: InstrumentationLibrary{
		Name: "an instrumentation library", Version: "an instrumentation library version",
	}, Metrics: []Metric{}}
	resourceMetrics := ResourceMetrics{
		ResourceMetrics: []ResourceMetric{
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm, ilm}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm, ilm, ilm}},
		},
	}
	expectedResourceMetrics := ResourceMetrics{
		ResourceMetrics: []ResourceMetric{
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm}},
		},
	}
	require.Equal(t, expectedResourceMetrics, FlattenResourceMetrics(resourceMetrics))
}

func TestFlattenResourceMetricsByMetricsIdentity(t *testing.T) {
	resource := Resource{Attributes: map[string]interface{}{}}
	metrics := []Metric{
		Metric{Name: "a metric", Unit: "a unit", Description: "a description", Value: 123},
		Metric{Name: "another metric", Unit: "another unit", Description: "another description", Value: 234},
		Metric{Name: "yet anothert metric", Unit: "yet anothe unit", Description: "yet anothet description", Value: 345},
	}
	ilm := InstrumentationLibraryMetrics{Metrics: metrics}
	ilmRepeated := InstrumentationLibraryMetrics{Metrics: append(metrics, metrics...)}
	ilmRepeatedTwice := InstrumentationLibraryMetrics{Metrics: append(metrics, append(metrics, metrics...)...)}
	resourceMetrics := ResourceMetrics{
		ResourceMetrics: []ResourceMetric{
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilmRepeated}},
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilmRepeatedTwice}},
		},
	}
	expectedResourceMetrics := ResourceMetrics{
		ResourceMetrics: []ResourceMetric{
			ResourceMetric{Resource: resource, ILMs: []InstrumentationLibraryMetrics{ilm}},
		},
	}
	require.Equal(t, expectedResourceMetrics, FlattenResourceMetrics(resourceMetrics))
}

func TestFlattenResourceMetricsIdempotent(t *testing.T) {
	resourceMetrics, err := PDataToResourceMetrics(pdataMetrics())
	require.NoError(t, err)
	require.NotNil(t, resourceMetrics)
	require.Equal(t, resourceMetrics, FlattenResourceMetrics(resourceMetrics))
	var rms []ResourceMetrics
	for i := 0; i < 100; i++ {
		rms = append(rms, resourceMetrics)
	}
	for i := 0; i < 100; i++ {
		require.Equal(t, resourceMetrics, FlattenResourceMetrics(rms...))
	}
}

func TestContainsAllEquivalenceCheck(t *testing.T) {
	resourceMetrics, err := LoadResourceMetrics(path.Join(".", "testdata", "resourceMetrics.yaml"))
	require.NoError(t, err)
	require.NotNil(t, resourceMetrics)

	containsAll, err := resourceMetrics.ContainsAll(resourceMetrics)
	require.True(t, containsAll, err)
}
