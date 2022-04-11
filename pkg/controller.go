// Copyright Aeraki Authors
//
// This file is mainly inspired by Istio xDS server
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

package pkg

import (
	"context"
	"fmt"
	"time"

	"istio.io/client-go/pkg/apis/networking/v1alpha3"

	"github.com/aeraki-framework/consul2istio/pkg/serviceregistry"
	"github.com/aeraki-framework/consul2istio/pkg/serviceregistry/consul"
	"github.com/gogo/protobuf/proto"
	istio "istio.io/api/networking/v1alpha3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	"istio.io/pkg/log"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// debounceAfter is the delay added to events to wait after a registry event for debouncing.
	// This will delay the push by at least this interval, plus the time getting subsequent events.
	// If no change is detected the push will happen, otherwise we'll keep delaying until things settle.
	debounceAfter = 500 * time.Millisecond

	// debounceMax is the maximum time to wait for events while debouncing.
	// Defaults to 10 seconds. If events keep showing up with no break for this time, we'll trigger a push.
	debounceMax = 10 * time.Second

	// configRootNS is the root config root namespace
	configRootNS = "istio-system"

	// aerakiFieldManager is the FileldManager for Aeraki CRDs
	aerakiFieldManager = "Aeraki"
)

type ChangeEvent struct {
}

type Controller struct {
	consulAddress string
	pushChannel   chan *ChangeEvent
	registry      serviceregistry.Registry
}

func NewController(consulAddress string) *Controller {
	controller := &Controller{
		consulAddress: consulAddress,
		pushChannel:   make(chan *ChangeEvent, 1),
	}
	return controller
}

// Run until a signal is received, this function won't block
func (s *Controller) Run(stop <-chan struct{}) error {
	log.Infof("Watch Consul at %s", s.consulAddress)
	if err := s.watchRegistry(stop); err != nil {
		log.Fatala(err)
		return err
	}
	go func() {
		s.mainLoop(stop)
	}()
	return nil
}

func (s *Controller) watchRegistry(stop <-chan struct{}) error {
	var err error
	s.registry, err = consul.NewController(s.consulAddress)
	if err != nil {
		return err
	}

	s.registry.AppendServiceChangeHandler(func() {
		s.pushChannel <- &ChangeEvent{}
	})
	//@todo gracefully close the registry controller
	s.registry.Run(stop)
	return nil
}

func (s *Controller) mainLoop(stop <-chan struct{}) {
	var timeChan <-chan time.Time
	var startDebounce time.Time
	var lastResourceUpdateTime time.Time
	pushCounter := 0
	debouncedEvents := 0

	for {
		select {
		case <-stop:
			break
		case e := <-s.pushChannel:
			log.Debugf("Receive event from push chanel : %v", e)
			lastResourceUpdateTime = time.Now()
			if debouncedEvents == 0 {
				log.Debugf("This is the first debounced event")
				startDebounce = lastResourceUpdateTime
			}
			timeChan = time.After(debounceAfter)
			debouncedEvents++
		case <-timeChan:
			log.Debugf("Receive event from time chanel")
			eventDelay := time.Since(startDebounce)
			quietTime := time.Since(lastResourceUpdateTime)
			// it has been too long since the first debounced event or quiet enough since the last debounced event
			if eventDelay >= debounceMax || quietTime >= debounceAfter {
				if debouncedEvents > 0 {
					pushCounter++
					log.Infof("Push debounce stable[%d] %d: %v since last change, %v since last push",
						pushCounter, debouncedEvents, quietTime, eventDelay)
					err := s.pushConsulService2APIServer()
					if err != nil {
						log.Errorf("Failed to synchronize consul services to Istio: %v", err)
						// Retry if failed
						s.pushChannel <- &ChangeEvent{}
					}
					debouncedEvents = 0
				}
			} else {
				timeChan = time.After(debounceAfter - quietTime)
			}
		}
	}
}

func (s *Controller) pushConsulService2APIServer() error {
	serviceEntries, err := s.registry.ServiceEntries()
	if err != nil {
		return fmt.Errorf("failed to get servcies from consul: %v", err)
	}

	newServiceEntries := make(map[string]*istio.ServiceEntry)
	for _, serviceEntry := range serviceEntries {
		newServiceEntries[serviceEntry.Hosts[0]] = serviceEntry
	}

	config, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("can not get kubernetes config: %v", err)
	}

	ic, err := versionedclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create istio client: %v", err)
	}

	existingServiceEntries, _ := ic.NetworkingV1alpha3().ServiceEntries(configRootNS).List(context.TODO(), v1.ListOptions{
		LabelSelector: "manager=" + aerakiFieldManager + ", registry=consul",
	})

	for _, oldServiceEntry := range existingServiceEntries.Items {
		if newServiceEntry, ok := newServiceEntries[oldServiceEntry.Spec.Hosts[0]]; !ok {
			log.Infof("Deleting EnvoyFilter: %s", oldServiceEntry.Name)
			err = ic.NetworkingV1alpha3().ServiceEntries(configRootNS).Delete(context.TODO(), oldServiceEntry.Spec.Hosts[0],
				v1.DeleteOptions{})
			if err != nil {
				err = fmt.Errorf("failed to create istio client: %v", err)
			}
		} else {
			if !proto.Equal(newServiceEntry, &oldServiceEntry.Spec) {
				log.Infof("Updating ServiceEntry: %v", *newServiceEntry)
				_, err = ic.NetworkingV1alpha3().ServiceEntries(configRootNS).Update(context.TODO(),
					toServiceEntryCRD(newServiceEntry, &oldServiceEntry),
					v1.UpdateOptions{FieldManager: aerakiFieldManager})
				if err != nil {
					err = fmt.Errorf("failed to update ServiceEntry: %v", err)
				}
			} else {
				log.Infof("ServiceEntry: %s unchanged", oldServiceEntry.Name)
			}
			delete(newServiceEntries, newServiceEntry.Hosts[0])
		}
	}

	for _, newServiceEntry := range newServiceEntries {
		_, err = ic.NetworkingV1alpha3().ServiceEntries(configRootNS).Create(context.TODO(), toServiceEntryCRD(newServiceEntry, nil),
			v1.CreateOptions{FieldManager: aerakiFieldManager})
		log.Infof("Creating ServiceEntry: %v", *newServiceEntry)
		if err != nil {
			err = fmt.Errorf("failed to create ServiceEntry: %v", err)
		}
	}
	return err
}

func toServiceEntryCRD(new *istio.ServiceEntry, old *v1alpha3.ServiceEntry) *v1alpha3.ServiceEntry {
	serviceEntry := &v1alpha3.ServiceEntry{
		ObjectMeta: v1.ObjectMeta{
			Name:      new.Hosts[0],
			Namespace: configRootNS,
			Labels: map[string]string{
				"manager":  aerakiFieldManager,
				"registry": "consul",
			},
		},
		Spec: *new,
	}
	if old != nil {
		serviceEntry.ResourceVersion = old.ResourceVersion
	}
	return serviceEntry
}
