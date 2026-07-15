// Nimos Library — shell nativo de escritorio (Wails v2).
//
// La app NO reimplementa nada: es una ventana WebView2 que arranca su propio
// backend (core :8090 + translate-wrap :8091) y hace de reverse-proxy a él. La
// página se carga desde el origen interno de Wails y todas las llamadas (/api,
// /content, /pdfjs) van relativas y proxeadas → MISMO ORIGEN de verdad, sin CORS.
//
// El ciclo de vida de los sidecars vive en sidecars.go; el splash en splash.go.
package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	sh, err := newShell()
	if err != nil {
		log.Fatalf("newShell: %v", err)
	}

	err = wails.Run(&options.App{
		Title:      "Nimos Library",
		Width:      1200,
		Height:     800,
		MinWidth:   900,
		MinHeight:  600,
		// Sin marco del SO: la SPA dibuja su propia barra (arrastre vía
		// --wails-draggable) y sus controles min/max/cerrar (window.runtime).
		Frameless:  true,
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
