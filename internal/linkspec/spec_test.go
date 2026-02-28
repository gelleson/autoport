package linkspec

import "testing"

func TestParseMany(t *testing.T) {
	specs, err := ParseMany([]string{
		"../svc-b/.env",
		"MONITORING_URL=../svc-b/.env:APP_PORT",
	})
	if err != nil {
		t.Fatalf("ParseMany() err: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d", len(specs))
	}
	if specs[0].Mode != ModeSmart || specs[0].EnvPath != "../svc-b/.env" {
		t.Fatalf("unexpected smart spec: %+v", specs[0])
	}
	if specs[1].Mode != ModeExplicit || specs[1].SourceKey != "MONITORING_URL" || specs[1].TargetPortKey != "APP_PORT" {
		t.Fatalf("unexpected explicit spec: %+v", specs[1])
	}
}

func TestParse_Invalid(t *testing.T) {
	if _, err := Parse("=../svc-b/.env"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := Parse("MONITORING_URL=../svc-b/.env:"); err == nil {
		t.Fatal("expected parse error")
	}
}
