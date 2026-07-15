// splash.go — pantalla de arranque servida mientras el core levanta.
// Sin assets externos (meta-refresh) para que funcione antes de que exista backend.
package main

import "net/http"

func serveSplash(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(splashHTML))
}

const splashHTML = `<!doctype html>
<html lang="es"><head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="1">
<title>Nimos Library</title>
<style>
  html,body{height:100%;margin:0}
  body{display:flex;flex-direction:column;align-items:center;justify-content:center;
       gap:18px;background:#0e0e14;color:#e9e9f0;
       font:15px/1.4 system-ui,Segoe UI,sans-serif;-webkit-user-select:none;user-select:none}
  .logo{font-size:44px}
  .name{font-size:20px;font-weight:600;letter-spacing:.2px}
  .msg{color:#83838f}
  .bar{width:180px;height:3px;border-radius:3px;overflow:hidden;background:#23232e;position:relative}
  .bar::after{content:"";position:absolute;inset:0;width:40%;border-radius:3px;
              background:linear-gradient(90deg,#7c6cf0,#9a8cff);animation:slide 1s ease-in-out infinite}
  @keyframes slide{0%{left:-40%}100%{left:100%}}
</style></head>
<body>
  <div class="logo">📚</div>
  <div class="name">Nimos Library</div>
  <div class="bar"></div>
  <div class="msg">Arrancando el servidor local…</div>
</body></html>`
