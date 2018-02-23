// Copyright 2018 Joonas Kuorilehto. All Rights Reserved.
// Copyright 2018 Twitch Interactive, Inc.  All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the License is
// located at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package prometheus

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/joneskoo/twirp-serverhook-prometheus/internal/twirptest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twitchtv/twirp"
)

var (
	registry = prometheus.NewRegistry()
)

func TestTimingHooks(t *testing.T) {
	hooks := NewServerHooks(registry)

	server, client := serverAndClient(hooks)
	defer server.Close()

	_, err := client.MakeHat(context.Background(), &twirptest.Size{})
	if err != nil {
		t.Fatalf("twirptest Client err=%q", err)
	}

	required := []string{"rpc_requests_total", "rpc_responses_total", "rpc_durations_seconds"}

	wantCounters := map[string][]counter{
		"rpc_requests_total": {
			counter{prometheus.Labels{"method": "MakeHat"}, 1},
		},
		"rpc_responses_total": {
			counter{prometheus.Labels{"method": "MakeHat", "status": "200"}, 1},
		},
	}

	wantSummaries := map[string][]summary{
		"rpc_durations_seconds": {
			summary{prometheus.Labels{"method": "MakeHat", "status": "200"}, 1},
		},
	}

	counters := checkCounters(t, wantCounters)
	summaries := checkSummaries(t, wantSummaries)
	got := append(counters, summaries...)

required:
	for _, name := range required {
		for _, x := range got {
			if name == x {
				continue required
			}
		}
		t.Errorf("Gather did not return required metric %v", name)
	}

}

func checkCounters(t *testing.T, expected map[string][]counter) []string {
	counters := []string{}
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry Gather returned error: %v", err)
	}
	for _, m := range metrics {
		metricType := m.GetType().String()
		if metricType != "COUNTER" {
			continue
		}
		name := m.GetName()
		counters = append(counters, name)

		want, ok := expected[name]
		if !ok {
			t.Errorf("got unexpected metric %v", name)
			continue
		}
		metrics := m.GetMetric()

		got := []counter{}
		for _, m := range metrics {
			labels := make(prometheus.Labels)
			for _, l := range m.Label {
				labels[l.GetName()] = l.GetValue()
			}
			value := m.GetCounter().GetValue()
			got = append(got, counter{labels, value})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v=%v, want %v", name, got, want)
		}
	}
	return counters
}

func checkSummaries(t *testing.T, expected map[string][]summary) []string {
	summaries := []string{}
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry Gather returned error: %v", err)
	}
	for _, m := range metrics {
		metricType := m.GetType().String()
		if metricType != "SUMMARY" {
			continue
		}
		name := m.GetName()
		summaries = append(summaries, name)

		want, ok := expected[name]
		if !ok {
			t.Errorf("got unexpected metric %v", name)
			continue
		}
		metrics := m.GetMetric()

		got := []summary{}
		for _, m := range metrics {
			labels := make(prometheus.Labels)
			for _, l := range m.Label {
				labels[l.GetName()] = l.GetValue()
			}
			samplecount := m.GetSummary().GetSampleCount()
			got = append(got, summary{labels, samplecount})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v=%v, want %v", name, got, want)
		}
	}
	return summaries
}

type counter struct {
	labels prometheus.Labels
	value  float64
}

type summary struct {
	labels      prometheus.Labels
	samplecount uint64
}

func serverAndClient(hooks *twirp.ServerHooks) (*httptest.Server, twirptest.Haberdasher) {
	return twirptest.ServerAndClient(twirptest.NoopHatmaker(), hooks)
}
