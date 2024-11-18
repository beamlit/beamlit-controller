# Beamlit Proxy

Beamlit Proxy is a configurable HTTP proxy used to handle traffic splitting inside a Kubernetes cluster.
Beamlit Proxy is part of the [Beamlit Operator](https://github.com/beamlit/beamlit-controller) project.

## Features

- Traffic splitting between any type of service (Kubernetes or not)
- Request/Response header manipulation
- Centralized authentication (basic auth, OAuth, JWT, API key)
- Caching
- Automatic retries
- Circuit breaking
- Healthchecks
- ...

## Build

```
docker build -t beamlit/beamlit-proxy:latest .
```

## Run

```
docker run -p 8080:8080 -p 8081:8081 beamlit/beamlit-proxy:latest
```

## API

See [API](api/README.md).
