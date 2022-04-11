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
	"time"

	"github.com/hashicorp/consul/api"

	"istio.io/pkg/log"
)

// Monitor handles service and instance changes
type Monitor interface {
	Start(<-chan struct{})
	AppendServiceChangeHandler(ServiceChangeHandler)
}

// ServiceChangeHandler processes service change events
// It's just a notification, we don't need to pass the changed services
type ServiceChangeHandler func() error

type consulMonitor struct {
	discovery             *api.Client
	ServiceChangeHandlers []ServiceChangeHandler
}

const (
	blockQueryWaitTime          time.Duration = 10 * time.Minute
	updateServiceRecordWaitTime time.Duration = 10 * time.Second
)

// NewConsulMonitor watches for changes in Consul services and CatalogServices
func NewConsulMonitor(client *api.Client) Monitor {
	return &consulMonitor{
		discovery:             client,
		ServiceChangeHandlers: make([]ServiceChangeHandler, 0),
	}
}

func (m *consulMonitor) Start(stop <-chan struct{}) {
	go m.watchConsul(stop)
}

func (m *consulMonitor) watchConsul(stop <-chan struct{}) {
	var consulWaitIndex uint64

	for {
		select {
		case <-stop:
			return
		default:
			queryOptions := api.QueryOptions{
				WaitIndex: consulWaitIndex,
				WaitTime:  blockQueryWaitTime,
			}
			// This Consul REST API will block until service changes or timeout
			// https://www.consul.io/api/features/blocking
			_, queryMeta, err := m.discovery.Catalog().Services(&queryOptions)
			if err != nil {
				log.Warnf("Could not fetch services: %v", err)
				time.Sleep(time.Second)
			} else if consulWaitIndex != queryMeta.LastIndex {
				consulWaitIndex = queryMeta.LastIndex
				m.updateServiceRecord()
				time.Sleep(updateServiceRecordWaitTime)
			}
		}
	}
}

func (m *consulMonitor) updateServiceRecord() {
	for _, f := range m.ServiceChangeHandlers {
		go func(handler ServiceChangeHandler) {
			if err := handler(); err != nil {
				log.Warnf("Error executing service handler function: %v", err)
			}
		}(f)
	}
}

func (m *consulMonitor) AppendServiceChangeHandler(h ServiceChangeHandler) {
	m.ServiceChangeHandlers = append(m.ServiceChangeHandlers, h)
}
