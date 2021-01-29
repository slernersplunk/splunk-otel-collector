# Splunk OpenTelemetry Collector Integration Tests and Utilities

To assist in vetting and validating the upstream and Splunk Collector distributions, this library provides a set of
integration tests and utilities.  The general testing pattern is based on:

1. Building the Collector (`make otelcol` or `make all`)
1. Defining your expected resource content as a yaml file ([see example](./testutils/testdata/resourceMetrics.yaml))
1. Standing up your target/source environments using the [helpers](./testutils/container.go) based on [testcontainers](https://pkg.go.dev/github.com/testcontainers/testcontainers-go)
1. Launching a [Collector subprocess](./testutils/collector_process.go) using your specified config file.
1. Launching an [in-memory OTLP Receiver and metric sink](./testutils/otlp_receiver_sink.go).
1. Asserting against received content.

At this time only limited metric content is supported.  If you need additional metrics or trace and log helpers, please help out!
