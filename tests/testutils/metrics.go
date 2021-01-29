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
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"gopkg.in/yaml.v2"
)

// Internal type for testing helpers and assertions.  Analogous to pdata form, with the exception that
// InstrumentationLibrary.Metrics items act as both metrics and metric datapoints (identity based on labels and values)
type ResourceMetrics struct {
	ResourceMetrics []ResourceMetric `yaml:"resource_metrics"`
}

type ResourceMetric struct {
	Resource Resource                        `yaml:",inline,omitempty"`
	ILMs     []InstrumentationLibraryMetrics `yaml:"ilms"`
}

type Resource struct {
	Attributes map[string]interface{} `yaml:"attributes,omitempty"`
}

type InstrumentationLibraryMetrics struct {
	InstrumentationLibrary InstrumentationLibrary `yaml:"instrumentation_library,omitempty"`
	Metrics                []Metric               `yaml:"metrics,omitempty"`
}

type InstrumentationLibrary struct {
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version,omitempty"`
}

type Metric struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Unit        string            `yaml:"unit,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Type        string            `yaml:"type"`
	Value       interface{}       `yaml:value,omitempty`
}

func LoadResourceMetrics(path string) (ResourceMetrics, error) {
	metricFile, err := os.Open(path)
	if err != nil {
		return ResourceMetrics{}, err
	}
	defer metricFile.Chdir()

	buffer := new(bytes.Buffer)
	_, err = buffer.ReadFrom(metricFile)
	by := buffer.Bytes()

	var loaded ResourceMetrics
	err = yaml.Unmarshal(by, &loaded)
	return loaded, err
}

func toInterfaceMap(stringMap map[string]string) map[string]interface{} {
	interfaceMap := map[string]interface{}{}
	for k, v := range stringMap {
		interfaceMap[k] = v
	}
	return interfaceMap
}

func mapHash(toHash map[string]interface{}) string {
	var keys []string
	for k, _ := range toHash {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var toJoin []string
	for _, k := range keys {
		toJoin = append(toJoin, fmt.Sprintf("%s:%v", k, toHash[k]))
	}
	return strings.Join(toJoin, ":")
}

func equivalentMaps(first, second map[string]interface{}) bool {
	pairs := []struct{ first, second map[string]interface{} }{{first, second}, {second, first}}
	for _, p := range pairs {
		for k, v := range p.first {
			if compareVal, ok := p.second[k]; !ok {
				return false
			} else {
				if v != compareVal {
					return false
				}
			}
		}
	}
	return true
}

func (resource Resource) Hash() string {
	hash := mapHash(resource.Attributes)
	return fmt.Sprintf("%x", md5.Sum([]byte(hash)))
}

func (resource Resource) Equals(toCompare Resource) bool {
	return equivalentMaps(resource.Attributes, toCompare.Attributes)
}

func (instrumentationLibrary InstrumentationLibrary) Hash() string {
	hash := fmt.Sprintf("%s:%s", instrumentationLibrary.Name, instrumentationLibrary.Version)
	return fmt.Sprintf("%x", md5.Sum([]byte(hash)))
}

func (instrumentationLibrary InstrumentationLibrary) Equals(toCompare InstrumentationLibrary) bool {
	return instrumentationLibrary.Name == toCompare.Name && instrumentationLibrary.Version == toCompare.Version
}

func (metric Metric) Hash() string {
	labelHash := mapHash(toInterfaceMap(metric.Labels))
	hash := fmt.Sprintf(
		"%v:%v:%v:%v:%v:%v:%v",
		metric.Name, metric.Description, metric.Unit,
		metric.Description, labelHash, metric.Type, metric.Value,
	)
	return fmt.Sprintf("%x", md5.Sum([]byte(hash)))
}

// Confirms that all define fields in receiver are matched in toCompare.
func (metric Metric) RelaxedEquals(toCompare Metric) bool {
	if metric.Name != "" && metric.Name != toCompare.Name {
		return false
	}
	if metric.Description != "" && metric.Description != toCompare.Description {
		return false
	}
	if metric.Unit != "" && metric.Unit != toCompare.Unit {
		return false
	}
	if metric.Type != "" && metric.Type != toCompare.Type {
		return false
	}
	if metric.Value != nil {
		reflectValue := reflect.ValueOf(metric.Value)
		if !reflectValue.IsZero() && metric.Value != toCompare.Value {
			return false
		}
	}
	if !equivalentMaps(toInterfaceMap(metric.Labels), toInterfaceMap(toCompare.Labels)) {
		return false
	}
	return true
}

// Ensures that everything in expectedResourceMetrics is in receiver (received ⊇ expected). Does not guarantee
// equivalence, or that expected contains all of received. Metric fields not in expected (e.g. unit, type, value, etc.)
// are not compared, but all labels must match.
// At this time calling on non-flattened ResourceMetrics is undefined.
func (receivedResourceMetrics ResourceMetrics) ContainsAll(expectedResourceMetrics ResourceMetrics) (bool, error) {
	requiredResources := len(expectedResourceMetrics.ResourceMetrics)
	containedResources := 0

	for _, resourceMetric := range receivedResourceMetrics.ResourceMetrics {
		for _, expectedResourceMetric := range expectedResourceMetrics.ResourceMetrics {
			if resourceMetric.Resource.Equals(expectedResourceMetric.Resource) {
				containedResources++
				requiredILMs := len(expectedResourceMetric.ILMs)
				containedILMs := 0
				for _, ilm := range resourceMetric.ILMs {
					for _, expectedILM := range expectedResourceMetric.ILMs {
						if ilm.InstrumentationLibrary.Equals(expectedILM.InstrumentationLibrary) {
							containedILMs++
							requiredMetrics := len(ilm.Metrics)
							containedMetrics := 0
							for _, metric := range ilm.Metrics {
								for _, expectedMetric := range expectedILM.Metrics {
									if expectedMetric.RelaxedEquals(metric) {
										containedMetrics++
									}
								}
							}
							if containedMetrics != requiredMetrics {
								return false, fmt.Errorf("%v doesn't contain all of %v", ilm.Metrics, expectedILM.Metrics)
							}
						}
					}
				}
				if containedILMs != requiredILMs {
					return false, fmt.Errorf("%v doesn't contain %v", resourceMetric.ILMs, expectedResourceMetric.ILMs)
				}
			}
		}
	}
	if requiredResources != containedResources {
		return false, fmt.Errorf("%v doesn't contain all of %v", receivedResourceMetrics.ResourceMetrics, expectedResourceMetrics.ResourceMetrics)
	}
	return true, nil
}

// FlattenResourceMetrics takes multiple instances of ResourceMetrics and flattens them
// to only unique entries by Resource, InstrumentationLibrary, and Metric content.
// It will preserve order through subsequent occurrences of repeated items being removed
// from the returned flattened ResourceMetrics
func FlattenResourceMetrics(resourceMetrics ...ResourceMetrics) ResourceMetrics {
	flattened := ResourceMetrics{}

	var resourceHashes []string
	// maps of resource hashes to objects
	resources := map[string]Resource{}
	ilms := map[string][]InstrumentationLibraryMetrics{}

	// flatten by Resource
	for _, rms := range resourceMetrics {
		for _, rm := range rms.ResourceMetrics {
			resourceHash := rm.Resource.Hash()
			if _, ok := resources[resourceHash]; !ok {
				resources[resourceHash] = rm.Resource
				resourceHashes = append(resourceHashes, resourceHash)
			}
			ilms[resourceHash] = append(ilms[resourceHash], rm.ILMs...)
		}
	}

	// flatten by InstrumentationLibrary
	for _, resourceHash := range resourceHashes {
		resource := resources[resourceHash]
		resourceMetric := ResourceMetric{
			Resource: resource,
		}

		var ilHashes []string
		// maps of hashes to objects
		ils := map[string]InstrumentationLibrary{}
		ilMetrics := map[string][]Metric{}
		for _, ilm := range ilms[resourceHash] {
			ilHash := ilm.InstrumentationLibrary.Hash()
			if _, ok := ils[ilHash]; !ok {
				ils[ilHash] = ilm.InstrumentationLibrary
				ilHashes = append(ilHashes, ilHash)
			}
			if ilm.Metrics == nil {
				ilm.Metrics = []Metric{}
			}
			ilMetrics[ilHash] = append(ilMetrics[ilHash], ilm.Metrics...)
		}

		// flatten by Metric
		for _, ilHash := range ilHashes {
			il := ils[ilHash]

			var metricHashes []string
			metrics := map[string]Metric{}
			allILMetrics := ilMetrics[ilHash]
			for _, metric := range allILMetrics {
				metricHash := metric.Hash()
				if _, ok := metrics[metricHash]; !ok {
					metrics[metricHash] = metric
					metricHashes = append(metricHashes, metricHash)
				}
			}

			var flattenedMetrics []Metric
			for _, metricHash := range metricHashes {
				flattenedMetrics = append(flattenedMetrics, metrics[metricHash])
			}

			if flattenedMetrics == nil {
				flattenedMetrics = []Metric{}
			}

			ilms := InstrumentationLibraryMetrics{
				InstrumentationLibrary: il,
				Metrics:                flattenedMetrics,
			}
			resourceMetric.ILMs = append(resourceMetric.ILMs, ilms)
		}

		flattened.ResourceMetrics = append(flattened.ResourceMetrics, resourceMetric)
	}

	//y, _ := yaml.Marshal(flattened)
	//fmt.Printf("%s\n", y)
	return flattened
}

func PDataToResourceMetrics(pdataMetrics ...pdata.Metrics) (ResourceMetrics, error) {
	resourceMetrics := ResourceMetrics{}
	for _, pdataMetric := range pdataMetrics {
		pdataRMs := pdataMetric.ResourceMetrics()
		numRM := pdataRMs.Len()
		for i := 0; i < numRM; i++ {
			rm := ResourceMetric{}
			pdataRM := pdataRMs.At(i)
			pdataRM.Resource().Attributes().ForEach(
				func(k string, v pdata.AttributeValue) {
					var val interface{}
					switch v.Type() {
					case pdata.AttributeValueSTRING:
						val = v.StringVal()
					case pdata.AttributeValueBOOL:
						val = v.BoolVal()
					case pdata.AttributeValueINT:
						val = v.IntVal()
					case pdata.AttributeValueDOUBLE:
						val = v.DoubleVal()
					case pdata.AttributeValueMAP:
						val = v.MapVal()
					case pdata.AttributeValueARRAY:
						val = v.ArrayVal()
					default:
						val = nil
					}
					if rm.Resource.Attributes == nil {
						rm.Resource.Attributes = map[string]interface{}{}
					}
					rm.Resource.Attributes[k] = val
				})
			pdataILMs := pdataRM.InstrumentationLibraryMetrics()
			for j := 0; j < pdataILMs.Len(); j++ {
				ilms := InstrumentationLibraryMetrics{Metrics: []Metric{}}
				pdataILM := pdataILMs.At(j)
				ilms.InstrumentationLibrary = InstrumentationLibrary{
					Name:    pdataILM.InstrumentationLibrary().Name(),
					Version: pdataILM.InstrumentationLibrary().Version(),
				}
				for k := 0; k < pdataILM.Metrics().Len(); k++ {
					pdataMetric := pdataILM.Metrics().At(k)
					switch pdataMetric.DataType() {
					case pdata.MetricDataTypeIntGauge:
						intGauge := pdataMetric.IntGauge()
						for l := 0; l < intGauge.DataPoints().Len(); l++ {
							dp := intGauge.DataPoints().At(l)
							val := dp.Value()
							labels := map[string]string{}
							dp.LabelsMap().ForEach(func(k, v string) {
								labels[k] = v

							})
							metric := Metric{
								Name:        pdataMetric.Name(),
								Description: pdataMetric.Description(),
								Unit:        pdataMetric.Unit(),
								Labels:      labels,
								Type:        "IntGauge",
								Value:       val,
							}
							ilms.Metrics = append(ilms.Metrics, metric)
						}
					case pdata.MetricDataTypeDoubleGauge:
						doubleGauge := pdataMetric.DoubleGauge()
						for l := 0; l < doubleGauge.DataPoints().Len(); l++ {
							dp := doubleGauge.DataPoints().At(l)
							val := dp.Value()
							labels := map[string]string{}
							dp.LabelsMap().ForEach(func(k, v string) {
								labels[k] = v

							})
							metric := Metric{
								Name:        pdataMetric.Name(),
								Description: pdataMetric.Description(),
								Unit:        pdataMetric.Unit(),
								Labels:      labels,
								Type:        "DoubleGauge",
								Value:       val,
							}
							ilms.Metrics = append(ilms.Metrics, metric)
						}
					case pdata.MetricDataTypeIntSum:
						intSum := pdataMetric.IntSum()
						metricType := "Int"
						if intSum.IsMonotonic() {
							metricType = fmt.Sprintf("%sMonotonic", metricType)
						} else {
							metricType = fmt.Sprintf("%sNonmonotonic", metricType)
						}
						switch intSum.AggregationTemporality() {
						case pdata.AggregationTemporalityCumulative:
							metricType = fmt.Sprintf("%sCumulative", metricType)
						case pdata.AggregationTemporalityDelta:
							metricType = fmt.Sprintf("%sDelta", metricType)
						case pdata.AggregationTemporalityUnspecified:
							metricType = fmt.Sprintf("%sUnspecified", metricType)
						}
						metricType = fmt.Sprintf("%sSum", metricType)
						for l := 0; l < intSum.DataPoints().Len(); l++ {
							dp := intSum.DataPoints().At(l)
							val := dp.Value()
							labels := map[string]string{}
							dp.LabelsMap().ForEach(func(k, v string) {
								labels[k] = v

							})
							metric := Metric{
								Name:        pdataMetric.Name(),
								Description: pdataMetric.Description(),
								Unit:        pdataMetric.Unit(),
								Labels:      labels,
								Type:        metricType,
								Value:       val,
							}
							ilms.Metrics = append(ilms.Metrics, metric)
						}
					case pdata.MetricDataTypeDoubleSum:
						doubleSum := pdataMetric.DoubleSum()
						metricType := "Double"
						if doubleSum.IsMonotonic() {
							metricType = fmt.Sprintf("%sMonotonic", metricType)
						} else {
							metricType = fmt.Sprintf("%sNonmonotonic", metricType)
						}
						switch doubleSum.AggregationTemporality() {
						case pdata.AggregationTemporalityCumulative:
							metricType = fmt.Sprintf("%sCumulative", metricType)
						case pdata.AggregationTemporalityDelta:
							metricType = fmt.Sprintf("%sDelta", metricType)
						case pdata.AggregationTemporalityUnspecified:
							metricType = fmt.Sprintf("%sUnspecified", metricType)
						}
						metricType = fmt.Sprintf("%sSum", metricType)
						for l := 0; l < doubleSum.DataPoints().Len(); l++ {
							dp := doubleSum.DataPoints().At(l)
							val := dp.Value()
							labels := map[string]string{}
							dp.LabelsMap().ForEach(func(k, v string) {
								labels[k] = v

							})
							metric := Metric{
								Name:        pdataMetric.Name(),
								Description: pdataMetric.Description(),
								Unit:        pdataMetric.Unit(),
								Labels:      labels,
								Type:        metricType,
								Value:       val,
							}
							ilms.Metrics = append(ilms.Metrics, metric)
						}
					case pdata.MetricDataTypeIntHistogram:
						panic(fmt.Sprintf("%s not yet supported", pdata.MetricDataTypeIntHistogram))
					case pdata.MetricDataTypeDoubleHistogram:
						panic(fmt.Sprintf("%s not yet supported", pdata.MetricDataTypeDoubleHistogram))
					case pdata.MetricDataTypeDoubleSummary:
						panic(fmt.Sprintf("%s not yet supported", pdata.MetricDataTypeDoubleSummary))
					default:
						panic(fmt.Sprintf("unexpected data type: %s", pdataMetric.DataType()))
					}
				}
				rm.ILMs = append(rm.ILMs, ilms)
			}
			resourceMetrics.ResourceMetrics = append(resourceMetrics.ResourceMetrics, rm)
		}
	}
	return resourceMetrics, nil
}
