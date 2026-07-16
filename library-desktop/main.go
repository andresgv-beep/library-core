// Nimos Library — shell nativo de escritorio (Wails v2).
//
// La app NO reimplementa nada: es una ventana WebView2 que se conecta a Library
// Server (local supervisado o remoto) y hace de reverse-proxy a él. La
// página se carga desde el origen interno de Wails y todas las llamadas (/api,
// /content, /pdfjs) van relativas y proxeadas → MISMO ORIGEN de verdad, sin CORS.
//
// El ciclo de vida del servidor pertenece a Library Supervisor. Esta interfaz
// nunca inicia ni detiene procesos.
package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// interfaceMode se fija a "panel" en el segundo ejecutable del todo-en-uno.
var interfaceMode = "client"

func main() {
	sh, err := newShell()
	if err != nil {
		log.Fatalf("newShell: %v", err)
	}

	title, width, height, minWidth, minHeight := "Nimos Library", 1200, 800, 900, 600
	if interfaceMode == "panel" {
		title, width, height, minWidth, minHeight = "Library Control Panel", 1040, 760, 760, 520
	}

	err = wails.Run(&options.App{
		Title:     title,
		Width:     width,
		Height:    height,
		MinWidth:  minWidth,
		MinHeight: minHeight,
		// Sin marco del SO: la SPA dibuja su propia barra (arrastre vía
		// --wails-draggable) y sus controles min/max/cerrar (window.runtime).
		Frameless: true,
		OnStartup:  sh.onStartup,
		OnShutdown: sh.onShutdown,
		// AssetServer.Handler recibe TODAS las peticiones de la webview: splash
		// mientras arranca, reverse-proxy al core una vez listo.
		AssetServer: &assetserver.Options{
			Handler: sh,
		},
	})
	if err != nil {
		log.Fatalf("wails.Run: %v", err)
	}
}
