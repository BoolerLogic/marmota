package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	goruntime "runtime"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"marmota/internal/bridge"
	"marmota/internal/proxy"
	"marmota/internal/repeater"
	"marmota/internal/settings"
)

// App struct
type App struct {
	ctx          context.Context
	settingsMu   sync.RWMutex
	initialState settings.InitialAppState
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	bridge.Init(ctx)

	config, loadErr := settings.Load(settings.ConfigFilePath)
	loadWarning := ""
	if loadErr != nil {
		loadWarning = fmt.Sprintf(
			"Could not load %s. Marmota is using safe defaults. Error: %v",
			settings.ConfigFilePath,
			loadErr,
		)
	}

	a.settingsMu.Lock()
	a.initialState = settings.InitialAppState{
		Config:            config,
		ConfigDirectory:   proxy.CADirectory,
		ConfigFilePath:    settings.ConfigFilePath,
		CACertificatePath: proxy.CACertPath,
		LoadWarning:       loadWarning,
	}
	a.settingsMu.Unlock()
}

func (a *App) shutdown(_ context.Context) {
	if proxy.IsProxyActive() {
		if err := proxy.CloseProxy(); err != nil {
			log.Printf("Could not close the proxy during shutdown: %v", err)
		}
	}
	bridge.Shutdown()
}

func (a *App) GetInitialAppState() settings.InitialAppState {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.initialState
}

func (a *App) OpenConfigDirectory() error {
	if err := os.MkdirAll(proxy.CADirectory, 0700); err != nil {
		return fmt.Errorf("create Marmota configuration directory: %w", err)
	}

	var command *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		command = exec.Command("explorer.exe", proxy.CADirectory)
	case "darwin":
		command = exec.Command("open", proxy.CADirectory)
	case "linux":
		command = exec.Command("xdg-open", proxy.CADirectory)
	default:
		return fmt.Errorf(
			"opening the configuration directory is not supported on %s",
			goruntime.GOOS,
		)
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("open Marmota configuration directory: %w", err)
	}
	return nil
}

func (a *App) ExportCA() (string, error) {
	_, _, err := proxy.GetOrCreateCA()
	if err != nil {
		return "", err
	}

	// 1. Abrir el diálogo de guardado
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export Marmota CA Certificate",
		DefaultFilename: "marmota-ca.crt",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Certificate (*.crt)", Pattern: "*.crt"},
			{DisplayName: "PEM (*.pem)", Pattern: "*.pem"},
		},
	})

	// Hubo un error real en el sistema
	if err != nil {
		return "", err
	}

	// El usuario cerró la ventana o le dio a cancelar
	if savePath == "" {
		return "", nil // Retornamos nil porque no es un fallo, simplemente abortamos la operación
	}

	// 2. Leer el certificado original de la ruta de configuración
	input, err := os.ReadFile(proxy.CACertPath)
	if err != nil {
		return "", err
	}

	// 3. Escribir el archivo en la nueva ubicación
	err = os.WriteFile(savePath, input, 0644) // 0644 da permisos de lectura para todos y escritura para el dueño
	if err != nil {
		return "", err
	}

	return savePath, nil
}

func (a *App) ResetCA() error {
	if proxy.IsProxyActive() {
		if err := proxy.CloseProxy(); err != nil {
			return err
		}
	}
	err := proxy.RemoveCAFiles()
	if err != nil {
		return err
	}
	_, _, err = proxy.GetOrCreateCA()
	return err
}

func (a *App) StartProxy(config settings.ProxyConfig) error {
	normalized, err := settings.Save(settings.ConfigFilePath, config)
	if err != nil {
		return err
	}

	runtimeConfig, err := normalized.RuntimeConfig()
	if err != nil {
		return err
	}

	a.settingsMu.Lock()
	a.initialState.Config = normalized
	a.initialState.LoadWarning = ""
	a.settingsMu.Unlock()

	return proxy.StartProxy(runtimeConfig)
}

func (a *App) CloseProxy() error {
	err := proxy.CloseProxy()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) SendRepeaterRequest(payload repeater.RepeaterSendPayload) (repeater.RepeaterSendResult, error) {
	return repeater.SendRepeaterRequest(payload)
}

func (a *App) GetHistoryEntryDetail(id uint64) bridge.HTTPHistoryEntryDetail {
	return bridge.GetHistoryEntryDetail(id)
}

func (a *App) UpsertActiveHistoryFilter(params bridge.UpsertActiveHistoryFilterParams) bridge.UpsertActiveHistoryFilterResult {
	return bridge.UpsertActiveHistoryFilter(params)
}

func (a *App) RemoveActiveHistoryFilter(params bridge.RemoveActiveHistoryFilterParams) {
	bridge.RemoveActiveHistoryFilter(params)
}

func (a *App) GetHistoryFilterMatchesForEntries(params bridge.GetHistoryFilterMatchesForEntriesParams) []bridge.HistoryEntryFilterMatches {
	return bridge.GetHistoryFilterMatchesForEntries(params)
}

func (a *App) RemoveHistoryEntry(id uint64) {
	bridge.RemoveHistoryEntry(id)
}

func (a *App) ClearHistoryEntries() {
	bridge.ClearHistoryEntries()
}
