package config

import "testing"

func TestValidate(t *testing.T) {
	type testCase struct {
		input   *Config
		wantErr bool
	}
	tcs := map[string]testCase{
		"When ProxyServiceRef Namesapce is nil, must return an error": {
			input: &Config{
				ProxyService: ProxyServiceConfig{
					Namespace: nil,
					Name:      toPointer("test"),
					Port:      toPointer(8080),
					AdminPort: toPointer(8081),
				},
			},
			wantErr: true,
		},
		"When ProxyServiceRef Name is nil, must return an error": {
			input: &Config{
				ProxyService: ProxyServiceConfig{
					Namespace: toPointer("namespace"),
					Name:      nil,
					Port:      toPointer(8080),
					AdminPort: toPointer(8081),
				},
			},
			wantErr: true,
		},
		"When ProxyServiceRef Port is nil, must return an error": {
			input: &Config{
				ProxyService: ProxyServiceConfig{
					Namespace: toPointer("namespace"),
					Name:      toPointer("test"),
					Port:      nil,
					AdminPort: toPointer(8081),
				},
			},
			wantErr: true,
		},
		"When ProxyServiceRef AdminPort is nil, must return an error": {
			input: &Config{
				ProxyService: ProxyServiceConfig{
					Namespace: toPointer("namespace"),
					Name:      toPointer("test"),
					Port:      toPointer(8080),
					AdminPort: nil,
				},
			},
			wantErr: true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			if err := tc.input.Validate(); (err != nil) != tc.wantErr {
				t.Errorf("wantErr is %v but errors is %v", tc.wantErr, (err != nil))
			}
		})
	}
}
