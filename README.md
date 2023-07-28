# Consul2Istio

[![CI Tests](https://github.com/aeraki-framework/consul2istio/workflows/ci/badge.svg?branch=master)](https://github.com/aeraki-framework/consul2istio/actions?query=branch%3Amaster+event%3Apush+workflow%3A%22ci%22)

Consul2istio watches Consul catalog and synchronize all the Consul services to Istio.

Consul2istio will create a ServiceEntry resource for each service in the Consul catalog.

![ consul2istio ](doc/consul2istio.png)

## example

Firstly, deploy consul and consul2istio to your Kubernetes cluster.

```bash
kubectl apply -f k8s/consul.yaml    
kubectl apply -f k8s/consul2istio.yaml                             
```

Secondly, deploy consumer-demo and provider-demo to your Kubernetes cluster. 
The consumer-demo service *9999/echo-rest/* to us.

```bash
kubectl apply -f k8s/sample.yaml                           
```

Finally, request from consumer-demo to provider-demo.
```
kubectl get pod -owide                                                                                
NAME                                READY   STATUS    RESTARTS        AGE     IP          NODE           NOMINATED NODE   READINESS GATES
consul-7bd648d9f-qpkrj              1/1     Running   0               58m     10.0.1.80   192.168.1.17   <none>           <none>
consul2istio-75c9dd98fd-cqglr       1/1     Running   0               9m32s   10.0.1.89   192.168.1.17   <none>           <none>
consumer-demo-66766c8d78-5stvw      2/2     Running   0               9m54s   10.0.1.88   192.168.1.17   <none>           <none>
provider-demo-v1-59ddd86974-z4sln   2/2     Running   0               9m54s   10.0.1.86   192.168.1.17   <none>           <none>
provider-demo-v2-5ccf64cdfd-dxdj5   2/2     Running   0               9m54s   10.0.1.87   192.168.1.17   <none>           <none>
```

the result of request.
```
kubectl exec -it consumer-demo-66766c8d78-5stvw  -c istio-proxy -- curl  10.0.1.88:9999/echo-rest/aaaa
echo() -> ip [ 10.0.1.86 ] param [ aaaa ]
```
