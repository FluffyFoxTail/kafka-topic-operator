# kafka-topic-operator
**Kafka Topic Operator** — is a Kubernetes operator designed to automate the management of Kafka topics via a custom resource `KafkaTopic`.

## Description
The operator creates, monitors, and updates Kafka topics using the [Sarama](https://github.com/Shopify/sarama) `ClusterAdmin API`.
It connects to Kafka brokers exposed through a Kubernetes `Service` and uses `EndpointSlice` resources to discover live brokers.

#### Example Manifest

```yaml
apiVersion: kafka.fluffyfoxtail.com/v1
kind: KafkaTopic
metadata:
  name: logs-kafka-topic
  namespace: monitoring
spec:
  replicas: 3
  requeueDelaySeconds: 15
  kafkaServiceName: kafka-broker
  kafkaServiceNamespace: monitoring
  kafkaPort: 9092
  topics:
    - name: access-logs
      partitions: 6
      replicationFactor: 3
    - name: error-logs
      partitions: 3
      replicationFactor: 2
```

#### Spec Fields

| Field                   | Type                      | Description                                                |
| ----------------------- | ------------------------- | ---------------------------------------------------------- |
| `replicas`              | `int32`                   | Number of Kafka brokers (informational, not used directly) |
| `requeueDelaySeconds`   | `int`                     | Delay before retrying reconciliation on error              |
| `kafkaServiceName`      | `string`                  | Kubernetes `Service` name exposing Kafka brokers           |
| `kafkaServiceNamespace` | `string`                  | Namespace where the Kafka `Service` is located             |
| `kafkaPort`             | `int32`                   | Kafka port (default is `9092`)                             |
| `topics`                | `[]KafkaTopicDescription` | List of Kafka topic descriptions                           |


## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kafka-topic-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kafka-topic-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```
>**NOTE**: Ensure that the samples has default values to test it out.

### To apply in specific namespace

```sh
kubectl apply -k config/default --namespace <your-namespace-name>
```

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/kafka-topic-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/kafka-topic-operator/<tag or branch>/dist/install.yaml
```
