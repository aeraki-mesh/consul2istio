// Copyright Aeraki Authors
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

package consul

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	istio "istio.io/api/networking/v1alpha3"

	"github.com/hashicorp/consul/api"
)

type mockServer struct {
	server      *httptest.Server
	services    map[string][]string
	productpage []*api.CatalogService
	reviews     []*api.CatalogService
	rating      []*api.CatalogService
	lock        sync.Mutex
	consulIndex int
}

func newServer() *mockServer {
	m := mockServer{
		productpage: []*api.CatalogService{
			{
				Node:           "istio-node",
				Address:        "172.19.0.5",
				ID:             "istio-node-id",
				ServiceID:      "productpage",
				ServiceName:    "productpage",
				ServiceTags:    []string{"version|v1"},
				ServiceAddress: "172.19.0.11",
				ServicePort:    9080,
			},
		},
		reviews: []*api.CatalogService{
			{
				Node:           "istio-node",
				Address:        "172.19.0.5",
				ID:             "istio-node-id",
				ServiceID:      "reviews-id",
				ServiceName:    "reviews",
				ServiceTags:    []string{"version|v1"},
				ServiceAddress: "172.19.0.6",
				ServicePort:    9081,
			},
			{
				Node:           "istio-node",
				Address:        "172.19.0.5",
				ID:             "istio-node-id",
				ServiceID:      "reviews-id",
				ServiceName:    "reviews",
				ServiceTags:    []string{"version|v2"},
				ServiceAddress: "172.19.0.7",
				ServicePort:    9081,
			},
			{
				Node:           "istio-node",
				Address:        "172.19.0.5",
				ID:             "istio-node-id",
				ServiceID:      "reviews-id",
				ServiceName:    "reviews",
				ServiceTags:    []string{"version|v3"},
				ServiceAddress: "172.19.0.8",
				ServicePort:    9080,
				ServiceMeta:    map[string]string{protocolTagName: "tcp"},
			},
		},
		rating: []*api.CatalogService{
			{
				Node:           "istio-node",
				Address:        "172.19.0.6",
				ID:             "istio-node-id",
				ServiceID:      "rating-id",
				ServiceName:    "rating",
				ServiceTags:    []string{"version|v1"},
				ServiceAddress: "172.19.0.12",
				ServicePort:    9080,
			},
		},
		services: map[string][]string{
			"productpage": {"version|v1"},
			"reviews":     {"version|v1", "version|v2", "version|v3"},
			"rating":      {"version|v1"},
		},
		consulIndex: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/catalog/services" {
			m.lock.Lock()
			data, _ := json.Marshal(&m.services)
			w.Header().Set("X-Consul-Index", strconv.Itoa(m.consulIndex))
			m.lock.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, string(data))
		} else if r.URL.Path == "/v1/catalog/service/reviews" {
			m.lock.Lock()
			data, _ := json.Marshal(&m.reviews)
			w.Header().Set("X-Consul-Index", strconv.Itoa(m.consulIndex))
			m.lock.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, string(data))
		} else if r.URL.Path == "/v1/catalog/service/productpage" {
			m.lock.Lock()
			data, _ := json.Marshal(&m.productpage)
			w.Header().Set("X-Consul-Index", strconv.Itoa(m.consulIndex))
			m.lock.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, string(data))
		} else if r.URL.Path == "/v1/catalog/service/rating" {
			m.lock.Lock()
			data, _ := json.Marshal(&m.rating)
			w.Header().Set("X-Consul-Index", strconv.Itoa(m.consulIndex))
			m.lock.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, string(data))
		} else {
			m.lock.Lock()
			data, _ := json.Marshal(&[]*api.CatalogService{})
			w.Header().Set("X-Consul-Index", strconv.Itoa(m.consulIndex))
			m.lock.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, string(data))
		}
	}))

	m.server = server
	return &m
}

func TestServiceEntries(t *testing.T) {
	ts := newServer()
	defer ts.server.Close()
	controller, err := NewController(ts.server.URL)
	if err != nil {
		t.Errorf("could not create Consul Controller: %v", err)
	}
	serviceEntries, err := controller.ServiceEntries()

	if err != nil {
		t.Errorf("client encountered error during ServiceEntries(): %v", err)
	}

	if len(serviceEntries) != 3 {
		t.Errorf("ServiceEntries() returned wrong number of service entry => %v, want 3", len(serviceEntries))
	}

	hostnames := []string{serviceHostname("productpage"), serviceHostname("reviews"), serviceHostname("rating")}
	services := map[string]*istio.ServiceEntry{}
	for _, serviceEntry := range serviceEntries {
		if len(serviceEntry.Hosts) == 1 {
			services[serviceEntry.Hosts[0]] = serviceEntry
		}
	}

	for _, host := range hostnames {
		if services[host] == nil {
			t.Errorf("Want host %v, but it's not in the result of ServiceEntries()", host)
		}
	}

	if len(services[serviceHostname("reviews")].Endpoints) != 3 {
		t.Errorf("ServiceEntries() get %v endpoints f, want 3", len(serviceEntries))
	}
}
