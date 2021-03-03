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
	"fmt"
	"testing"

	istio "istio.io/api/networking/v1alpha3"

	"github.com/hashicorp/consul/api"

	"istio.io/istio/pkg/config/protocol"
)

var (
	protocols = []struct {
		name string
		port int
		out  protocol.Instance
	}{
		{"tcp", 80, protocol.TCP},
		{"http", 81, protocol.HTTP},
		{"https", 443, protocol.HTTPS},
		{"http2", 83, protocol.HTTP2},
		{"grpc", 84, protocol.GRPC},
		{"udp", 85, protocol.UDP},
		{"", 86, protocol.TCP},
	}

	goodLabels = []string{
		"key1|val1",
		"version|v1",
	}

	badLabels = []string{
		"badtag",
		"goodtag|goodvalue",
	}
)

func TestConvertProtocol(t *testing.T) {
	for _, tt := range protocols {
		out := convertPort(tt.port, tt.name)
		if out.Protocol != string(tt.out) {
			t.Errorf("convertProtocol(%v, %q) => %q, want %q", tt.port, tt.name, out, tt.out)
		}
	}
}

func TestConvertLabels(t *testing.T) {
	out := convertLabels(goodLabels)
	if len(out) != len(goodLabels) {
		t.Errorf("convertLabels(%q) => length %v, want %v", goodLabels, len(out), len(goodLabels))
	}

	out = convertLabels(badLabels)
	if len(out) == len(badLabels) {
		t.Errorf("convertLabels(%q) => length %v, want %v", badLabels, len(out), len(badLabels)-1)
	}
}

func TestServiceHostname(t *testing.T) {
	out := serviceHostname("productpage")

	if out != "productpage.service.consul" {
		t.Errorf("serviceHostname() => %q, want %q", out, "productpage.service.consul")
	}
}

func TestConvertWorkloadEntry(t *testing.T) {
	ip := "172.19.0.11"
	port := 9080
	p := "udp"
	name := "productpage"
	tagKey1 := "version"
	tagVal1 := "v1"
	tagKey2 := "zone"
	tagVal2 := "prod"
	dc := "dc1"
	consulServiceInst := api.CatalogService{
		Node:        "istio-node",
		Address:     "172.19.0.5",
		ID:          "1111-22-3333-444",
		ServiceName: name,
		ServiceTags: []string{
			fmt.Sprintf("%v|%v", tagKey1, tagVal1),
			fmt.Sprintf("%v|%v", tagKey2, tagVal2),
		},
		ServiceAddress: ip,
		ServicePort:    port,
		Datacenter:     dc,
		ServiceMeta:    map[string]string{protocolTagName: p},
	}

	out := convertWorkloadEntry(&consulServiceInst)

	if out.Ports[p] != 9080 {
		t.Errorf("convertWorkloadEntry() => %v, want %v", out.Ports[p], protocol.UDP)
	}

	if out.Locality != dc {
		t.Errorf("convertWorkloadEntry() => %v, want %v", out.Locality, dc)
	}

	if out.Address != ip {
		t.Errorf("convertWorkloadEntry() => %v, want %v", out.Address, ip)
	}

	if len(out.Labels) != 2 {
		t.Errorf("convertWorkloadEntry() len(Labels) => %v, want %v", len(out.Labels), 2)
	}

	if out.Labels[tagKey1] != tagVal1 || out.Labels[tagKey2] != tagVal2 {
		t.Errorf("convertWorkloadEntry() => missing or incorrect tag in %q", out.Labels)
	}
}

func TestConverServiceEntry(t *testing.T) {
	name := "productpage"
	port := 9080
	p := "udp"

	consulServiceInsts := []*api.CatalogService{
		{
			Node:        "istio-node",
			Address:     "172.19.0.5",
			ID:          "1111-22-3333-444",
			ServiceName: name,
			ServiceTags: []string{
				"version=v1",
				"zone=prod",
			},
			ServiceAddress: "172.19.0.11",
			ServicePort:    port,
			ServiceMeta:    map[string]string{protocolTagName: p},
		},
		{
			Node:        "istio-node",
			Address:     "172.19.0.5",
			ID:          "1111-22-3333-444",
			ServiceName: name,
			ServiceTags: []string{
				"version=v2",
			},
			ServiceAddress: "172.19.0.12",
			ServicePort:    port,
			ServiceMeta:    map[string]string{protocolTagName: p},
		},
	}

	out := convertServiceEntry(name, consulServiceInsts)

	if len(out.Endpoints) != 2 {
		t.Errorf("converServiceEntry() len(Endpoints) => %v, want %v", len(out.Endpoints), 2)
	}

	if out.Location == istio.ServiceEntry_MESH_EXTERNAL {
		t.Error("converServiceEntry() should not be an external service")
	}

	if len(out.Hosts) != 1 {
		t.Errorf("converServiceEntry() len(Hosts) => %v, want %v", len(out.Hosts), 0)
	}

	if out.Hosts[0] != serviceHostname(name) {
		t.Errorf("converServiceEntry() bad hostname => %q, want %q",
			out.Hosts[0], serviceHostname(name))
	}

	if out.Resolution != istio.ServiceEntry_STATIC {
		t.Errorf("converServiceEntry() incorrect resolution => %v, want %v", out.Resolution, istio.ServiceEntry_STATIC)
	}

	//we assume there's no virtual IP for consul service
	if len(out.Addresses) != 0 {
		t.Errorf("converServiceEntry() len(Addresses) => %v, want %v", len(out.Addresses), 0)
	}

	if len(out.Ports) != 1 {
		t.Errorf("converServiceEntry() incorrect # of ports => %v, want %v",
			len(out.Ports), 1)
	}

	if out.Ports[0].Number != uint32(port) {
		t.Errorf("converServiceEntry() => %v, want %v", out.Ports[0].Number, protocol.UDP)
	}

	if out.Ports[0].Name != p {
		t.Errorf("converServiceEntry() => %v, want %v", out.Ports[0].Name, p)
	}
}
