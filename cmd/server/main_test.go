package main

import "testing"

func TestParseConfigPath(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "default path",
			args: nil,
			want: defaultConfigPath,
		},
		{
			name: "custom path",
			args: []string{"--config", "config/appliance.yaml"},
			want: "config/appliance.yaml",
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
		{
			name:    "unexpected positional arg",
			args:    []string{"config/custom.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfigPath(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
