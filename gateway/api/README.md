# API

## Version management

Handling versioning is done by the API server. The API server will only serve the latest version of the API.

## Version v1alpha1

### Routes

#### Register Route

```
POST /v1alpha1/routes

{
    "name": "my-route",
    "hostnames": ["example.com"],
    "backends": [
        {
            "host": "example.com:8080",
            "weight": 2,
            "scheme": "https"
        },
        {
            "addr": "example.com:8081",
            "weight": 1,
            "scheme": "http"
        }
    ]
}
```

#### Get Route

```
GET /v1alpha1/routes/{name}
```

#### Delete Route

```
DELETE /v1alpha1/routes/{name}
```

#### Update Route

```
PUT /v1alpha1/routes/{name}

{
    "name": "my-route",
    "hostnames": ["example.com"],
    "backends": [
        ...
    ]
}
```
