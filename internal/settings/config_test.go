package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"marmota/internal/proxy"
)

func TestLoadMissingConfigurationUsesDefaults(t *testing.T) {
	config, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("load missing configuration: %v", err)
	}
	if !reflect.DeepEqual(config, DefaultProxyConfig()) {
		t.Fatalf("configuration = %#v, want defaults %#v", config, DefaultProxyConfig())
	}
}

func TestSaveLoadAndOverwriteConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	first := ProxyConfig{
		SchemaVersion:        currentSchemaVersion,
		ProxyMode:            ProxyModeSpecific,
		SpecificIP:           " 192.168.1.50 ",
		Port:                 9090,
		SkipServerCertVerify: true,
		UpstreamProxy: proxy.UpstreamProxyConfig{
			Enabled:  true,
			Host:     " proxy.example ",
			Port:     1080,
			Username: "user",
			Password: "password",
		},
	}

	normalized, err := Save(path, first)
	if err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	if normalized.SpecificIP != "192.168.1.50" ||
		normalized.UpstreamProxy.Host != "proxy.example" {
		t.Fatalf("configuration was not normalized: %#v", normalized)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load saved configuration: %v", err)
	}
	if !reflect.DeepEqual(loaded, normalized) {
		t.Fatalf("loaded configuration = %#v, want %#v", loaded, normalized)
	}

	second := DefaultProxyConfig()
	second.ProxyMode = ProxyModeAll
	second.Port = 8181
	if _, err := Save(path, second); err != nil {
		t.Fatalf("overwrite configuration: %v", err)
	}

	loaded, err = Load(path)
	if err != nil {
		t.Fatalf("load overwritten configuration: %v", err)
	}
	if !reflect.DeepEqual(loaded, second) {
		t.Fatalf("overwritten configuration = %#v, want %#v", loaded, second)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat saved configuration: %v", err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("configuration permissions = %o, want user-only", info.Mode().Perm())
		}
	}
}

func TestLoadInvalidConfigurationUsesDefaultsAndReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"proxyMode":"invalid"}`), 0600); err != nil {
		t.Fatalf("write invalid configuration: %v", err)
	}

	config, err := Load(path)
	if err == nil {
		t.Fatal("invalid configuration did not return an error")
	}
	if !reflect.DeepEqual(config, DefaultProxyConfig()) {
		t.Fatalf("invalid configuration fallback = %#v, want defaults", config)
	}
}

func TestSaveRejectsInvalidListenerPort(t *testing.T) {
	config := DefaultProxyConfig()
	config.Port = 0

	if _, err := Save(filepath.Join(t.TempDir(), "config.json"), config); err == nil {
		t.Fatal("zero listener port did not return an error")
	}
}

func TestLoadRejectsOversizedConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(
		path,
		[]byte(strings.Repeat("x", maxConfigFileSize+1)),
		0600,
	); err != nil {
		t.Fatalf("write oversized configuration: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("oversized configuration did not return an error")
	}
}
