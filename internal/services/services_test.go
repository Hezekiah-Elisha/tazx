package services

import "testing"

func TestValidateServiceName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"nginx", "nginx.service", false},
		{"postgresql.service", "postgresql.service", false},
		{"php8.2-fpm", "php8.2-fpm.service", false},
		{"docker@1", "docker@1.service", false},
		{"", "", true},
		{"-rf", "", true},
		{"--all", "", true},
		{"nginx; rm -rf /", "", true},
		{"../../etc/passwd", "", true},
		{"svc name", "", true},
	}

	for _, tc := range tests {
		got, err := ValidateServiceName(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateServiceName(%q) err = %v, wantErr = %v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ValidateServiceName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseUnitListAndUnitFiles(t *testing.T) {
	unitOutput := `
nginx.service loaded active running A high performance web server
broken.service loaded failed failed Broken worker daemon
`
	units := ParseUnitList(unitOutput)
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	if units[0].Name != "broken.service" || units[0].ActiveState != "failed" {
		t.Errorf("unexpected first unit: %+v", units[0])
	}
	if units[1].Name != "nginx.service" || units[1].Description != "A high performance web server" {
		t.Errorf("unexpected second unit: %+v", units[1])
	}

	ufOutput := `
nginx.service enabled enabled
broken.service disabled enabled
`
	states := ParseUnitFiles(ufOutput)
	if states["nginx.service"] != "enabled" || states["broken.service"] != "disabled" {
		t.Errorf("unexpected unit file states: %+v", states)
	}
}

func TestParseServiceShow(t *testing.T) {
	showOutput := `Id=nginx.service
LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
Description=A high performance web server
MainPID=1234
ActiveEnterTimestamp=Mon 2026-11-23 10:00:00 UTC
`
	info, err := ParseServiceShow(showOutput)
	if err != nil {
		t.Fatalf("ParseServiceShow failed: %v", err)
	}
	if info.Name != "nginx.service" || info.MainPID != 1234 || info.Enabled != "enabled" {
		t.Errorf("unexpected ServiceInfo: %+v", info)
	}
}
