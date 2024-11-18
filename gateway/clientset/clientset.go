package clientset

import "net/http"

type ClientSet struct {
	apiAddr        string
	client         *http.Client
	v1Alpha1Client V1Alpha1Client
}

func NewClientSet(client *http.Client, apiAddr string) *ClientSet {
	cl := &ClientSet{
		apiAddr: apiAddr,
		client:  client,
	}
	cl.v1Alpha1Client = cl
	return cl
}
