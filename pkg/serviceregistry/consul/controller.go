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
	"sync"

	"github.com/hashicorp/consul/api"
	istio "istio.io/api/networking/v1alpha3"
	"istio.io/pkg/log"
)

// Controller communicates with Consul and monitors for changes
type Controller struct {
	client       *api.Client
	monitor      Monitor
	servicesList []*istio.ServiceEntry
	initDone     bool
	cacheMutex   sync.Mutex
}

// NewController creates a new Consul controller
func NewController(addr string) (*Controller, error) {
	conf := api.DefaultConfig()
	conf.Address = addr

	client, err := api.NewClient(conf)
	monitor := NewConsulMonitor(client)
	controller := Controller{
		monitor:      monitor,
		client:       client,
		servicesList: make([]*istio.ServiceEntry, 0),
	}

	//Watch the change events to refresh local caches
	monitor.AppendServiceChangeHandler(controller.ServiceChanged)
	return &controller, err
}

// Run until a stop signal is received
func (c *Controller) Run(stop <-chan struct{}) {
	c.monitor.Start(stop)
}

// Services list declarations of all services in the system
func (c *Controller) ServiceEntries() ([]*istio.ServiceEntry, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	return c.servicesList, nil
}

// AppendServiceHandler implements a service catalog operation
func (c *Controller) AppendServiceChangeHandler(serviceChanged func()) {
	c.monitor.AppendServiceChangeHandler(func() error {
		serviceChanged()
		return nil
	})
}

func (c *Controller) initCache() error {
	if c.initDone {
		return nil
	}

	// get all services from consul
	consulServices, err := c.getServices()
	if err != nil {
		return err
	}
	c.servicesList = nil
	for serviceName := range consulServices {
		// get endpoints of a service from consul
		endpoints, err := c.getCatalogService(serviceName, nil)
		if err != nil {
			return nil
		}
		c.servicesList = append(c.servicesList, convertServiceEntry(serviceName, endpoints))
	}

	c.initDone = true
	return nil
}

func (c *Controller) getServices() (map[string][]string, error) {
	data, _, err := c.client.Catalog().Services(nil)
	if err != nil {
		log.Warnf("Could not retrieve services from consul: %v", err)
		return nil, err
	}
	return data, nil
}

func (c *Controller) getCatalogService(name string, q *api.QueryOptions) ([]*api.CatalogService, error) {
	endpoints, _, err := c.client.Catalog().Service(name, "", q)
	if err != nil {
		log.Warnf("Could not retrieve service catalog from consul: %v", err)
		return nil, err
	}
	return endpoints, nil
}

func (c *Controller) ServiceChanged() error {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.initDone = false
	return nil
}
