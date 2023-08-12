// Copyright Aeraki Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"istio.io/pkg/log"

	"github.com/aeraki-framework/consul2istio/pkg"
	"github.com/aeraki-framework/consul2istio/pkg/constants"
	"github.com/aeraki-framework/consul2istio/pkg/serviceregistry/consul"
)

func main() {
	args := consul.NewConsulBootStrapArgs()

	flag.StringVar(&args.ConsulAddress, "consulAddress", constants.DefaultConsulAddress, "Consul Address")
	flag.StringVar(&args.Namespace, "namespace", constants.ConfigRootNS, "namespace")
	flag.StringVar(&args.FQDN, "fqdn", "", "The FQDN for consul service")
	flag.BoolVar(&args.EnableDefaultPort, "enableDefaultPort", true,
		"The flag to start default port for consul service")

	flag.Parse()

	flag.VisitAll(func(flag *flag.Flag) {
		log.Infof("consul2istio parameter: %s: %v", flag.Name, flag.Value)
	})

	initArgsWithEnv(args)
	log.Infof("consul2istio bootstrap parameter: %v", args)

	controller := pkg.NewController(args)

	// Create the stop channel for all of the servers.
	stopChan := make(chan struct{}, 1)
	err := controller.Run(stopChan)
	if err != nil {
		log.Errorf("Fialed to run controller: %v", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	stopChan <- struct{}{}
}

func initArgsWithEnv(args *consul.BootStrapArgs) {
	consulAddress := os.Getenv("consulAddress")
	if consulAddress != "" {
		args.ConsulAddress = consulAddress
	}

	namespace := os.Getenv("namespace")
	if namespace != "" {
		args.Namespace = namespace
	}
}
