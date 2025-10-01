package config

import (
	"os"
	"testing"
)

func TestPopulateDMRPasswordPerBridge(t *testing.T) {
	defer os.Unsetenv("BRIDGE_TESTBRIDGE_DMR_PASSWORD")
	os.Setenv("BRIDGE_TESTBRIDGE_DMR_PASSWORD", "secret123")

	cfg := &Config{
		Bridges: []BridgeConfig{
			{Name: "testbridge", DMR: &DMRBridgeConfig{}},
		},
	}

	populateDMRPasswordsFromEnv(cfg)

	if cfg.Bridges[0].DMR.Password != "secret123" {
		t.Fatalf("expected password to be populated from env, got %q", cfg.Bridges[0].DMR.Password)
	}
}

func TestPopulateDMRPasswordGlobalYSF2DMR(t *testing.T) {
	defer os.Unsetenv("YSF2DMR_DMR_PASSWORD")
	os.Setenv("YSF2DMR_DMR_PASSWORD", "globalpw")

	cfg := &Config{
		YSF2DMR: YSF2DMRConfig{DMR: YSF2DMRDMRConfig{}},
	}

	populateDMRPasswordsFromEnv(cfg)

	if cfg.YSF2DMR.DMR.Password != "globalpw" {
		t.Fatalf("expected global ysf2dmr password populated, got %q", cfg.YSF2DMR.DMR.Password)
	}
}

func TestSanitization(t *testing.T) {
	defer os.Unsetenv("BRIDGE_BRANDMEISTER_TG91_DMR_PASSWORD")
	os.Setenv("BRIDGE_BRANDMEISTER_TG91_DMR_PASSWORD", "pw")

	cfg := &Config{
		Bridges: []BridgeConfig{
			{Name: "BrandMeister TG91", DMR: &DMRBridgeConfig{}},
		},
	}

	populateDMRPasswordsFromEnv(cfg)

	if cfg.Bridges[0].DMR.Password != "pw" {
		t.Fatalf("expected sanitized env var to populate password, got %q", cfg.Bridges[0].DMR.Password)
	}
}

func TestDoesNotOverwriteExisting(t *testing.T) {
	defer os.Unsetenv("BRIDGE_EXISTING_DMR_PASSWORD")
	os.Setenv("BRIDGE_EXISTING_DMR_PASSWORD", "should-not-use")

	cfg := &Config{
		Bridges: []BridgeConfig{
			{Name: "existing", DMR: &DMRBridgeConfig{Password: "already"}},
		},
	}

	populateDMRPasswordsFromEnv(cfg)

	if cfg.Bridges[0].DMR.Password != "already" {
		t.Fatalf("expected existing password not to be overwritten, got %q", cfg.Bridges[0].DMR.Password)
	}
}
