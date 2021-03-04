# Consul2Istio

[![CI Tests](https://github.com/aeraki-framework/consul2istio/workflows/ci/badge.svg?branch=master)](https://github.com/aeraki-framework/consul2istio/actions?query=branch%3Amaster+event%3Apush+workflow%3A%22ci%22)

Consul2istio watches Consul catalog and synchronize all the Consul services to Istio.

Consul2istio will create a ServiceEntry resource for each service in the Consul catalog.

![ consul2istio ](doc/consul2istio.png)