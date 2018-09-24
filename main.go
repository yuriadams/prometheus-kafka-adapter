// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The main package for the Prometheus server executable.
package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/yuriadams/prometheus-kafka-adapter/config"
	"github.com/yuriadams/prometheus-kafka-adapter/writer"
)

func init() {
	prometheus.MustRegister(config.ReceivedSamples)
	prometheus.MustRegister(config.SentSamples)
	prometheus.MustRegister(config.FailedSamples)
	prometheus.MustRegister(config.SentBatchDuration)
	prometheus.MustRegister(config.WriteSamples)
	prometheus.MustRegister(config.ReadDuration)
	prometheus.MustRegister(config.ReadErrors)
}

func main() {
	cfg := config.GetConfig()
	http.Handle(cfg.TelemetryPath, prometheus.Handler())

	http.HandleFunc("/write", writer.Handle)

	log.Info("Starting server %s...", cfg.ListenAddr)
	http.ListenAndServe(cfg.ListenAddr, nil)
}
