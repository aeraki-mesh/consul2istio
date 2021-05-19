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
	"strings"

	"github.com/hashicorp/consul/api"

	"istio.io/pkg/log"

	istio "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/config/protocol"
)

const (
	protocolTagName = "protocol"
	externalTagName = "external"
)

func convertServiceEntry(service string, endpoints []*api.CatalogService) *istio.ServiceEntry {
	name := ""
	location := istio.ServiceEntry_MESH_INTERNAL
	resolution := istio.ServiceEntry_STATIC
	ports := make(map[uint32]*istio.Port)
	workloadEntries := make([]*istio.WorkloadEntry, 0)

	for _, endpoint := range endpoints {
		name = endpoint.ServiceName
		port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])

		if svcPort, exists := ports[port.Number]; exists && svcPort.Protocol != port.Protocol {
			log.Warnf("Service %v has two instances on same port %v but different protocols (%v, %v)",
				name, port.Number, svcPort.Protocol, port.Protocol)
		} else {
			ports[port.Number] = port
		}

		// TODO：This will not work if service is a mix of external and local services or if a service has more than one external name
		if endpoint.ServiceMeta[externalTagName] != "" {
			location = istio.ServiceEntry_MESH_EXTERNAL
			resolution = istio.ServiceEntry_NONE
		}

		workloadEntries = append(workloadEntries, convertWorkloadEntry(endpoint))
	}

	svcPorts := make([]*istio.Port, 0, len(ports))
	for _, port := range ports {
		svcPorts = append(svcPorts, port)
	}

	hostname := serviceHostname(service)
	out := &istio.ServiceEntry{
		Hosts:      []string{hostname},
		Ports:      svcPorts,
		Location:   location,
		Resolution: resolution,
		Endpoints:  workloadEntries,
	}

	return out
}

func convertWorkloadEntry(endpoint *api.CatalogService) *istio.WorkloadEntry {
	svcLabels := convertLabels(endpoint.ServiceTags)

	addr := endpoint.ServiceAddress
	if addr == "" {
		addr = endpoint.Address
	}

	// 如果是 DNS 需要解析出域名
	//if net.ParseIP(addr) == nil {
	//	ip, err := net.LookupIP(addr)
	//	if err != nil {
	//		log.Errorf("Lookup IP error: %v", err)
	//	} else {
	//		addr = ip[0].String()
	//	}
	//}

	port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])

	return &istio.WorkloadEntry{
		Address:  addr,
		Ports:    map[string]uint32{port.Name: port.Number},
		Labels:   svcLabels,
		Locality: endpoint.Datacenter,
	}
}

func convertLabels(labelsStr []string) labels.Instance {
	out := make(labels.Instance, len(labelsStr))

	for _, tag := range labelsStr {
		values := strings.Split(tag, "|")
		// Labels not of form "key|value" are ignored to avoid possible collisions
		if len(values) > 1 {
			out[values[0]] = values[1]
		} else {
			log.Debugf("Tag %v ignored since it is not of form key|value", tag)
		}
	}

	return out
}

func convertPort(port int, name string) *istio.Port {
	if name == "" {
		name = "tcp"
	}

	return &istio.Port{
		Number:     uint32(port),
		Protocol:   convertProtocol(name),
		Name:       name,
		TargetPort: uint32(port),
	}
}

// serviceHostname produces FQDN for a consul service
func serviceHostname(name string) string {
	// a DNS-1123 subdomain must consist of lower case alphanumeric characters,
	// '-' or '.', and must start and end with an alphanumeric character.

	//name = strings.ToLower(name)
	//if strings.Contains(name, "_") {
	//	name = strings.ReplaceAll(name, "_", "-")
	//}

	// TODO include datacenter in Hostname?
	// consul DNS uses "redis.service.us-east-1.consul" -> "[<optional_tag>].<svc>.service.[<optional_datacenter>].consul"
	return fmt.Sprintf("%s.service.consul", name)
}

func convertProtocol(name string) string {
	p := protocol.Parse(name)
	if p == protocol.Unsupported {
		log.Warnf("unsupported protocol value: %s", name)
		return string(protocol.TCP)
	}
	return string(p)
}
