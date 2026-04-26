package main

import (
	"context"
	"marmota/internal/bridge"
	"marmota/internal/proxy"
	"marmota/internal/repeater"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
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
}

func (a *App) ExportCA() (string, error) {
	_, _, err := proxy.GetOrCreateCA()
	if err != nil {
		return "", err
	}

	// 1. Abrir el diálogo de guardado
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Marmota CA Certificate",
		DefaultFilename: "marmota-ca.crt",
		Filters: []runtime.FileFilter{
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

func (a *App) StartProxy(ip string, port uint16, skipServerCertVerify bool) error {
	err := proxy.StartProxy(proxy.ConfigProxy{IP: ip, Port: port, SkipServerCertVerify: skipServerCertVerify})
	if err != nil {
		return err
	}
	return nil
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
