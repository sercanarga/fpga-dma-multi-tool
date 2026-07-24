package main

import "testing"

func TestDriverINFMatchesOriginalPackageName(t *testing.T) {
	tests := []struct {
		name        string
		contents    string
		originalINF string
		want        bool
	}{
		{
			name:        "WCH",
			contents:    `[SourceDisksFiles]\nCH341WDM.SYS=1`,
			originalINF: "CH341WDM.INF",
			want:        true,
		},
		{
			name:        "FTDI",
			contents:    `[Strings]\nServiceName="ftdibus3"`,
			originalINF: "ftdibus3.inf",
			want:        true,
		},
		{
			name:        "other package",
			contents:    `[Strings]\nServiceName="usbser"`,
			originalINF: "ftdibus3.inf",
			want:        false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := driverINFMatches([]byte(test.contents), test.originalINF); got != test.want {
				t.Fatalf("driverINFMatches() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSetupRequiredWhenAnyDriverIsMissing(t *testing.T) {
	installed := []componentStatus{
		{Name: "CH347 driver", Installed: true},
		{Name: "FTDI D3XX driver", Installed: true},
	}
	if setupRequired(installed) {
		t.Fatal("setupRequired() = true when all drivers are installed")
	}
	installed[1].Installed = false
	if !setupRequired(installed) {
		t.Fatal("setupRequired() = false when a driver is missing")
	}
}
