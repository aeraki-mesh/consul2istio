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
	"net"
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

	migrateHostName = "hostname"
	migrateTags     = "tags"
	migrateAppId    = "appid"
)

var ServiceZone string

type MigrateTag struct {
	Hostname string `json:"hostname"`
	Backlog  int    `json:"backlog"`
	Stats    struct {
		T string `json:"t"`
		Y int    `json:"y"`
		N int    `json:"n"`
	} `json:"stats"`
	AppId  string `json:"appid"`
	Weight int    `json:"weight"`
	Tags   string `json:"tags"`
}

func convertServiceEntry(service string, endpoints []*api.CatalogService) *istio.ServiceEntry {
	location := istio.ServiceEntry_MESH_INTERNAL
	resolution := istio.ServiceEntry_STATIC
	ports := make([]*istio.Port, 0)
	workloadEntries := make([]*istio.WorkloadEntry, 0)

	for _, endpoint := range endpoints {
		port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])

		//if svcPort, exists := ports[port.Number]; exists && svcPort.Protocol != port.Protocol {
		//	log.Warnf("Service %v has two instances on same port %v but different protocols (%v, %v)",
		//		endpoint.ServiceName, port.Number, svcPort.Protocol, port.Protocol)
		//} else {
		//	ports[port.Number] = port
		//}

		if len(ports) == 0 {
			ports = append(ports, port)
		}

		// TODO：This will not work if service is a mix of external and local services or if a service has more than one external name
		if endpoint.ServiceMeta[externalTagName] != "" {
			location = istio.ServiceEntry_MESH_EXTERNAL
			resolution = istio.ServiceEntry_NONE
		}

		workloadEntries = append(workloadEntries, convertWorkloadEntry(endpoint))
	}

	//svcPorts := make([]*istio.Port, 0, len(ports))
	//for _, port := range ports {
	//	svcPorts = append(svcPorts, port)
	//}

	out := &istio.ServiceEntry{
		Hosts:      []string{serviceHostname(service)},
		Ports:      ports,
		Location:   location,
		Resolution: resolution,
		Endpoints:  workloadEntries,
	}

	return out
}

func convertWorkloadEntry(endpoint *api.CatalogService) *istio.WorkloadEntry {
	svcLabels, weight := convertLabels(endpoint.ServiceTags)

	// If on service address exists or it is not a ip address, use the node address
	addr := endpoint.ServiceAddress
	if net.ParseIP(addr) == nil {
		addr = endpoint.Address
	}

	port := convertPort(endpoint.ServicePort, endpoint.ServiceMeta[protocolTagName])

	return &istio.WorkloadEntry{
		Address:  addr,
		Ports:    map[string]uint32{port.Name: port.Number},
		Labels:   svcLabels,
		Locality: ServiceZone,
		Weight:   weight,
	}
}

func convertLabels(labelsStr []string) (labels.Instance, uint32) {
	out := make(labels.Instance, len(labelsStr))
	weight := uint32(0)

	for _, tag := range labelsStr {
		// Labels not of form "key|value" are ignored to avoid possible collisions
		if strings.Contains(tag, "|") {
			values := strings.Split(tag, "|")
			if len(values) > 1 {
				out[values[0]] = values[1]
			}
			continue
		}

		// Labels start with '{' maybe migrateTags
		if strings.Contains(tag, "{") {
			migrateTag := new(MigrateTag)
			if err := json.Unmarshal([]byte(tag), migrateTag); err != nil {
				log.Debugf("Tag %v ignored since it is not the migrateTag", tag)
				continue
			}
			out[migrateHostName] = migrateTag.Hostname
			out[migrateAppId] = migrateTag.AppId
			out[migrateTags] = migrateTag.Tags
			weight = uint32(migrateTag.Weight)
		}
	}

	return out, weight
}

func convertPort(port int, name string) *istio.Port {
	// Default is TCP protocol
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

	// For DNS-1123 only contains '-', '.' and low case letter
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ToLower(name)

	// TODO: include datacenter in Hostname?
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
