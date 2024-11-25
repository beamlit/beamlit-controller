package proxy

import "testing"

func Test_extractHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{
			name: "Host with a port must return the host",
			host: "example.com:8080",
			want: "example.com",
		},
		{
			name: "Host without a port must return the host",
			host: "example.com",
			want: "example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractHost(tt.host); got != tt.want {
				t.Errorf("extractHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
