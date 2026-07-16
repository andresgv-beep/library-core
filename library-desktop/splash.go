package main

import (
	"html/template"
	"net/http"
)

func serveSplash(w http.ResponseWriter, remote bool, target string) {
	message := "Conectando con el servicio local de Library Server..."
	if remote {
		message = "Conectando con " + target + "..."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(pageStart + `<meta http-equiv="refresh" content="1"><title>Nimos Library</title>` + pageStyle + `</head><body><main><img src="data:image/svg+xml,` + escapedLogo + `" alt=""><h1>Nimos Library</h1><div class="bar"></div><p>` + template.HTMLEscapeString(message) + `</p></main></body></html>`))
}

func serveSetup(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(pageStart + `<title>Conectar Nimos Library</title>` + pageStyle + `</head><body><main class="setup"><img src="data:image/svg+xml,` + escapedLogo + `" alt=""><h1>Conecta tu Library Server</h1><p>Escribe la direccion del equipo o NAS que guarda tu biblioteca.</p><form id="setup"><input id="target" type="url" required autofocus placeholder="https://library.ejemplo.local"><button>Conectar</button><small id="error">` + template.HTMLEscapeString(message) + `</small></form></main><script>
document.getElementById('setup').addEventListener('submit',async function(event){
 event.preventDefault();var button=this.querySelector('button'),error=document.getElementById('error');button.disabled=true;error.textContent='Comprobando...';
 try{var response=await fetch('/__nimos/gateway',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({target:document.getElementById('target').value})});var body=await response.json();if(!response.ok)throw new Error(body.error||'No se pudo guardar');location.reload();}
 catch(e){error.textContent=e.message;button.disabled=false;}
});
</script></body></html>`))
}

const pageStart = `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">`

const pageStyle = `<style>
html,body{height:100%;margin:0}body{display:grid;place-items:center;background:#0e0e14;color:#e9e9f0;font:15px/1.45 system-ui,Segoe UI,sans-serif}main{width:min(440px,calc(100% - 48px));display:flex;flex-direction:column;align-items:center;text-align:center;gap:14px}img{width:82px;height:82px}h1{font-size:22px;margin:0}p{color:#9393a0;margin:0}.bar{width:190px;height:3px;border-radius:3px;overflow:hidden;background:#23232e;position:relative}.bar:after{content:"";position:absolute;inset:0;width:40%;border-radius:3px;background:linear-gradient(90deg,#7c6cf0,#f0468a);animation:slide 1s ease-in-out infinite}@keyframes slide{0%{left:-40%}100%{left:100%}}form{width:100%;display:flex;flex-direction:column;gap:11px;margin-top:12px}input,button{box-sizing:border-box;width:100%;height:46px;border-radius:11px;font:inherit}input{border:1px solid #353543;background:#181820;color:#fff;padding:0 14px;outline:none}input:focus{border-color:#8b5cf6}button{border:0;background:linear-gradient(135deg,#6f5ee8,#9b4fe1);color:#fff;font-weight:650;cursor:pointer}button:disabled{opacity:.55;cursor:wait}small{min-height:20px;color:#f08094}
</style>`

// El SVG va escapado para poder usarse como data URL sin depender del servidor.
const escapedLogo = `%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512'%3E%3Cdefs%3E%3ClinearGradient id='a' x2='1' y2='1'%3E%3Cstop stop-color='%234a7df0'/%3E%3Cstop offset='.5' stop-color='%238b5cf6'/%3E%3Cstop offset='1' stop-color='%23f0468a'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cg fill='url(%23a)'%3E%3Ccircle cx='184' cy='232' r='64'/%3E%3Ccircle cx='256' cy='188' r='86'/%3E%3Ccircle cx='332' cy='220' r='76'/%3E%3Crect x='128' y='230' width='256' height='112' rx='56'/%3E%3C/g%3E%3Cpath fill='%23db2777' d='M306 262q10 0 6 11l-22 52h42q13 0 5 13l-87 114q-8 10-5-5l22-66h-42q-13 0-7-12l44-104q4-8 11-11Z'/%3E%3C/svg%3E`
