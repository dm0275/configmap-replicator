# ConfigMap Replicator Controller
## Overview
The ConfigMap Replicator controller can be used to simplify the replication of ConfigMaps across namespaces. 
This controller allows users to easily copy, synchronize, or replicate ConfigMaps from one namespace to one or more target namespaces.

## Features

- **Easy Replication**: Replicate ConfigMaps across multiple namespaces.
- **Fine-Grained Control**: Specify which namespaces to include or exclude from replication.
- **Annotation-Based Configuration**: Control replication behavior using simple Kubernetes annotations.


## Installation
The `configmap-replicator` controller can be installed via Helm. To install, add the Helm chart repository:

```bash
helm repo add cm-repo https://dm0275.github.io/configmap-replicator \
  && helm repo update
```

Install the latest version of the controller by running:

```bash
helm install configmap-replicator cm-repo/configmap-replicator
```

### Helm Chart Values

Below is a table with the values available in the Helm chart:

| Parameter                           | Description                                                       | Default Value                 |
|-------------------------------------|-------------------------------------------------------------------|-------------------------------|
| `replicaCount`                      | Number of replicas for the controller                             | `1`                           |
| `image.pullPolicy`                  | Image pull policy                                                 | `IfNotPresent`                |
| `image.tag`                         | Image tag (defaults to chart appVersion)                          | `""`                          |
| `imagePullSecrets`                  | List of image pull secrets                                        | `[]`                          |
| `nameOverride`                      | Override name of the chart                                        | `""`                          |
| `fullnameOverride`                  | Override full name of the chart                                   | `""`                          |
| `rbac.create`                       | Create RBAC(Role & RoleBinding) configurations for the controller | `true`                        |
| `rbac.serviceAccount.name`          | Name of the ServiceAccount                                        | `""`                          |
| `rbac.clusterRole.name`             | Name of the ClusterRole                                           | `""`                          |
| `rbac.clusterRoleBinding.name`      | Name of the ClusterRoleBinding                                    | `""`                          |
| `serviceAccount.create`             | Create a new ServiceAccount                                       | `true`                        |
| `serviceAccount.automount`          | Automount ServiceAccount API credentials                          | `true`                        |
| `serviceAccount.annotations`        | Annotations for the ServiceAccount                                | `{}`                          |
| `serviceAccount.name`               | ServiceAccount name                                               | `""`                          |
| `podAnnotations`                    | Annotations for the pod                                           | `{}`                          |
| `podLabels`                         | Labels for the pod                                                | `{}`                          |
| `resources.limits.cpu`              | CPU limit for the container                                       | `1`                           |
| `resources.limits.memory`           | Memory limit for the container                                    | `128Mi`                       |
| `resources.requests.cpu`            | CPU request for the container                                     | `100m`                        |
| `resources.requests.memory`         | Memory request for the container                                  | `128Mi`                       |
| `nodeSelector`                      | Node selector for pod scheduling                                  | `{}`                          |
| `tolerations`                       | Tolerations for pod scheduling                                    | `[]`                          |
| `affinity`                          | Affinity rules for pod scheduling                                 | `{}`                          |
| `replicator.reconciliationInterval` | Interval for ConfigMap reconciliation                             | `1m`                          |

## Usage

Once deployed, the `configmap-replicator` will automatically replicate ConfigMaps based on the annotations you specify.

### Enable Replication

To enable replication of a ConfigMap across all namespaces, add the following annotation to your ConfigMap:

```yaml
annotations:
  configmap-replicator/replication-allowed: "true"
```

### Exclude Specific Namespaces

If you want to exclude specific namespaces from replication, use the following annotation:

```yaml
annotations:
  configmap-replicator/replication-allowed: "true"
  configmap-replicator/excluded-namespaces: "kube-system"
```

### Replicate to Specific Namespaces Only

To replicate the ConfigMap to a specific set of namespaces, use this annotation:

```yaml
annotations:
  configmap-replicator/replication-allowed: "true"
  configmap-replicator/allowed-namespaces: "team1,team2"
```

## Build from source
To build the `configmap-replicator` controller, follow the steps below:

* Build the controller using Gradle:

   ```bash
   gradle build
   ```


## License

This project is licensed under the Apache License. See the [LICENSE](LICENSE.txt) file for details.
