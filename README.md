# ConfigMap Replicator Controller
## Overview
The ConfigMap Replicator controller can be used to simplify the replication of ConfigMaps across namespaces. 
This controller allows users to easily copy, synchronize, or replicate ConfigMaps from one namespace to one or more target namespaces.

## Features

- **Easy Replication**: Replicate ConfigMaps across multiple namespaces.
- **Fine-Grained Control**: Specify which namespaces to include or exclude from replication.
- **Annotation-Based Configuration**: Control replication behavior using simple Kubernetes annotations.

## Installation
TODO

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
