# Beamlit Controller Code Architecture

The Beamlit Controller is a Kubernetes operator that provides a way to manage Beamlit applications. It is built using the Operator SDK and the Kubernetes Go client library.

## Controller package

Controller package is the core of the controller. It is responsible for watching the resources and reconciling the desired state with the actual state.

To do this, it uses a Kubernetes controller pattern. It watches the Beamlit applications and ordinates the creation of the corresponding elements.
It also keeps listening informers for the metrics and health probes in order to update the dataplane accordingly.

## Dataplane package

In this package, we implement the dataplane logic. This is the part that is responsible for the actual network setup for offloading.

## Informers package

This package contains the informers that are used to watch various elements. Such as metrics and health probes.
