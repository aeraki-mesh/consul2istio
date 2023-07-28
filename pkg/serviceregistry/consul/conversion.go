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
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
	istio "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/pkg/log"
)

const (
	protocolTagName    = "protocol"
	externalTagName    = "external"
	defaultServicePort = 80
)

func convertServiceEntry(enableDefaultPort bool, fqdn, service string, endpoints []*api.CatalogService) *istio.ServiceEntry {
	name := ""
	location := istio.ServiceEntry_MESH_INTERNAL
	resolution := istio.ServiceEntry_STATIC
	ports := make(map[uint32]*istio.Port)
	workloadEntries := make([]*istio.WorkloadEntry, 0)

	for _, endpoint := range endpoints {
		name = endpoint.ServiceName

		port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])

		if svcPort, exists := ports[port.Number]; exists && svcPort.Protocol != port.Protocol {
			log.Infof("Service %v has two instances on same port %v but different protocols (%v, %v)",
				name, port.Number, svcPort.Protocol, port.Protocol)
		} else {
			ports[port.Number] = port
		}
		if enableDefaultPort {
			ports[defaultServicePort] = convertPort(defaultServicePort, "")
		}

		// TODO This will not work if service is a mix of external and local services
		// or if a service has more than one external name
		if endpoint.ServiceMeta[externalTagName] != "" {
			location = istio.ServiceEntry_MESH_EXTERNAL
			resolution = istio.ServiceEntry_NONE
		}

		workloadEntries = append(workloadEntries, convertWorkloadEntry(enableDefaultPort, endpoint))
	}

	svcPorts := make([]*istio.Port, 0, len(ports))
	for _, port := range ports {
		svcPorts = append(svcPorts, port)
	}

	hostname := serviceHostname(service, fqdn)
	out := &istio.ServiceEntry{
		Hosts:      []string{hostname},
		Ports:      svcPorts,
		Location:   location,
		Resolution: resolution,
		Endpoints:  workloadEntries,
	}
	return out
}

func convertWorkloadEntry(enableDefaultPort bool, endpoint *api.CatalogService) *istio.WorkloadEntry {
	svcLabels := convertLabels(endpoint.ServiceTags)
	addr := endpoint.ServiceAddress
	if addr == "" {
		addr = endpoint.Address
	}
	ports := make(map[string]uint32, 0)

	port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])
	ports[port.Name] = port.Number

	if enableDefaultPort {
		defaultPort := convertPort(defaultServicePort, "")
		ports[defaultPort.Name] = port.Number
	}

	return &istio.WorkloadEntry{
		Address:  addr,
		Ports:    ports,
		Labels:   svcLabels,
		Locality: endpoint.Datacenter,
	}
}

func convertLabels(labelsStr []string) labels.Instance {
	out := make(labels.Instance, len(labelsStr))
	for _, tag := range labelsStr {
		vals := strings.Split(tag, "|")
		// Labels not of form "key|value" are ignored to avoid possible collisions
		if len(vals) > 1 {
			out[vals[0]] = vals[1]
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

	sport := strconv.Itoa(port)
	protocol := convertProtocol(name)
	return &istio.Port{
		Number:     uint32(port),
		Protocol:   protocol,
		Name:       name + "-" + sport,
		TargetPort: uint32(port),
	}
}

// serviceHostname produces FQDN for a consul service
func serviceHostname(name, fqdn string) string {
	// TODO include datacenter in Hostname?
	// consul DNS uses "redis.service.us-east-1.consul" -> "[<optional_tag>].<svc>.service.[<optional_datacenter>].consul"
	if len(fqdn) > 0 {
		return fmt.Sprintf("%s.%s", name, fqdn)
	}
	return name
}

func convertProtocol(name string) string {
	p := protocol.Parse(name)
	if p == protocol.Unsupported {
		log.Infof("unsupported protocol value: %s", name)
		return string(protocol.TCP)
	}
	return string(p)
}
