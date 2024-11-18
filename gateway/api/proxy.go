package api

import (
	"net/http"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
)

type Proxy interface {
	http.Handler
	v1alpha1.Proxy
}
