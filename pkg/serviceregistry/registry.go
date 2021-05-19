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

package serviceregistry

import istio "istio.io/api/networking/v1alpha3"

// Controller defines an event controller loop. Proxy agent registers itself
// with the controller loop and receives notifications on changes to the
// service topology or changes to the configuration artifacts.
//
// The controller guarantees the following consistency requirement: registry
// view in the controller is as AT LEAST as fresh as the moment notification
// arrives, but MAY BE more fresh (e.g. "delete" cancels an "add" event).  For
// example, an event for a service creation will see a service registry without
// the service if the event is immediately followed by the service deletion
// event.
//
// Handlers execute on the single worker queue in the order they are appended.
// Handlers receive the notification event and the associated object.  Note
// that all handlers must be appended before starting the controller.
type Controller interface {
	// AppendServiceChangeHandler notifies about changes to the service catalog.
	AppendServiceChangeHandler(serviceChanged func())

	// Run until a signal is received
	Run(stop <-chan struct{})
}

type ServiceDiscovery interface {
	// ServiceEntries list declarations of all services in this registry
	ServiceEntries() ([]*istio.ServiceEntry, error)
}

type Registry interface {
	Controller
	ServiceDiscovery
}
