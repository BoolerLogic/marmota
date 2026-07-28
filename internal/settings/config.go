package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"marmota/internal/proxy"
)

const (
	currentSchemaVersion = 1
	maxConfigFileSize    = 64 * 1024
)

type ProxyMode string

const (
	ProxyModeLocal    ProxyMode = "local"
	ProxyModeAll      ProxyMode = "all"
	ProxyModeSpecific ProxyMode = "specific"
)

var ConfigFilePath = filepath.Join(proxy.CADirectory, "config.json")

type ProxyConfig struct {
	SchemaVersion        int                       `json:"schemaVersion"`
	ProxyMode            ProxyMode                 `json:"proxyMode"`
	SpecificIP           string                    `json:"specificIp"`
	Port                 uint16                    `json:"port"`
	SkipServerCertVerify bool                      `json:"skipServerCertVerify"`
	UpstreamProxy        proxy.UpstreamProxyConfig `json:"upstreamProxy"`
}

type InitialAppState struct {
	Config            ProxyConfig `json:"config"`
	ConfigDirectory   string      `json:"configDirectory"`
	ConfigFilePath    string      `json:"configFilePath"`
	CACertificatePath string      `json:"caCertificatePath"`
	LoadWarning       string      `json:"loadWarning"`
}

func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		SchemaVersion: currentSchemaVersion,
		ProxyMode:     ProxyModeLocal,
		Port:          8080,
		UpstreamProxy: proxy.UpstreamProxyConfig{
			Port: 1080,
		},
	}
}

func Normalize(config ProxyConfig) (ProxyConfig, error) {
	if config.SchemaVersion == 0 {
		config.SchemaVersion = currentSchemaVersion
	}
	if config.SchemaVersion != currentSchemaVersion {
		return ProxyConfig{}, fmt.Errorf(
			"unsupported Marmota configuration schema version %d",
			config.SchemaVersion,
		)
	}

	config.ProxyMode = ProxyMode(strings.ToLower(strings.TrimSpace(
		string(config.ProxyMode),
	)))
	config.SpecificIP = strings.TrimSpace(config.SpecificIP)
	config.UpstreamProxy.Host = strings.TrimSpace(config.UpstreamProxy.Host)

	if config.ProxyMode != ProxyModeLocal &&
		config.ProxyMode != ProxyModeAll &&
		config.ProxyMode != ProxyModeSpecific {
		return ProxyConfig{}, fmt.Errorf(
			"invalid proxy listen mode %q",
			config.ProxyMode,
		)
	}

	if _, err := config.RuntimeConfig(); err != nil {
		return ProxyConfig{}, err
	}
	return config, nil
}

func (config ProxyConfig) RuntimeConfig() (proxy.ConfigProxy, error) {
	var listenIP string
	switch config.ProxyMode {
	case ProxyModeLocal:
		listenIP = "127.0.0.1"
	case ProxyModeAll:
		listenIP = "0.0.0.0"
	case ProxyModeSpecific:
		listenIP = strings.TrimSpace(config.SpecificIP)
	default:
		return proxy.ConfigProxy{}, fmt.Errorf(
			"invalid proxy listen mode %q",
			config.ProxyMode,
		)
	}

	runtimeConfig := proxy.ConfigProxy{
		IP:                   listenIP,
		Port:                 config.Port,
		SkipServerCertVerify: config.SkipServerCertVerify,
		UpstreamProxy:        config.UpstreamProxy,
	}
	if err := proxy.ValidateConfig(runtimeConfig); err != nil {
		return proxy.ConfigProxy{}, err
	}
	return runtimeConfig, nil
}

func Load(path string) (ProxyConfig, error) {
	defaultConfig := DefaultProxyConfig()

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig, nil
	}
	if err != nil {
		return defaultConfig, fmt.Errorf("open Marmota configuration: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxConfigFileSize+1))
	if err != nil {
		return defaultConfig, fmt.Errorf("read Marmota configuration: %w", err)
	}
	if len(data) > maxConfigFileSize {
		return defaultConfig, fmt.Errorf(
			"Marmota configuration exceeds %d bytes",
			maxConfigFileSize,
		)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return defaultConfig, errors.New("Marmota configuration is empty")
	}

	var config ProxyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return defaultConfig, fmt.Errorf("parse Marmota configuration: %w", err)
	}

	normalized, err := Normalize(config)
	if err != nil {
		return defaultConfig, fmt.Errorf("validate Marmota configuration: %w", err)
	}
	return normalized, nil
}

func Save(path string, config ProxyConfig) (ProxyConfig, error) {
	normalized, err := Normalize(config)
	if err != nil {
		return ProxyConfig{}, err
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("encode Marmota configuration: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return ProxyConfig{}, fmt.Errorf("create Marmota configuration directory: %w", err)
	}

	tempFile, err := os.CreateTemp(directory, ".config-*.tmp")
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("create temporary Marmota configuration: %w", err)
	}
	tempPath := tempFile.Name()
	removeTempFile := true
	defer func() {
		if removeTempFile {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return ProxyConfig{}, fmt.Errorf("protect temporary Marmota configuration: %w", err)
	}
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return ProxyConfig{}, fmt.Errorf("write temporary Marmota configuration: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return ProxyConfig{}, fmt.Errorf("sync temporary Marmota configuration: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return ProxyConfig{}, fmt.Errorf("close temporary Marmota configuration: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return ProxyConfig{}, fmt.Errorf("replace Marmota configuration: %w", err)
	}
	removeTempFile = false

	return normalized, nil
}
