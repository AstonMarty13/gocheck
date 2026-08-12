// Package web renders the gocheck dashboard.
package web

import (
	"html/template"
	"net/http"
	"time"

	"github.com/AstonMarty13/gocheck/internal/monitor"
)

var tmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.Format("15:04:05")
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="10">
  <title>gocheck dashboard</title>
  <style>
    body { font-family: sans-serif; padding: 40px; background: #f5f5f5; }
    h1   { margin-bottom: 24px; }
    table { border-collapse: collapse; width: 100%; background: white;
            box-shadow: 0 1px 4px rgba(0,0,0,.1); border-radius: 8px; overflow: hidden; }
    th   { background: #333; color: white; padding: 12px 16px; text-align: left; }
    td   { padding: 10px 16px; border-bottom: 1px solid #eee; }
    tr:last-child td { border-bottom: none; }
    .up   { color: #2e7d32; font-weight: bold; }
    .down { color: #c62828; font-weight: bold; }
    small { color: #888; }
  </style>
</head>
<body>
  <h1>gocheck dashboard</h1>
  <table>
    <thead>
      <tr><th>Site</th><th>Status</th><th>HTTP code</th><th>Response time</th><th>Last check</th></tr>
    </thead>
    <tbody>
    {{range .}}
      <tr>
        <td><a href="{{.URL}}" target="_blank" rel="noopener noreferrer">{{.URL}}</a></td>
        <td>{{if .Up}}<span class="up">UP</span>{{else}}<span class="down">DOWN</span>{{end}}</td>
        <td>{{if .StatusCode}}{{.StatusCode}}{{else}}—{{end}}</td>
        <td>{{if .LastChecked.IsZero}}—{{else}}{{.ResponseMS}} ms{{end}}</td>
        <td><small>{{formatTime .LastChecked}}</small></td>
      </tr>
    {{end}}
    </tbody>
  </table>
  <p><small>Auto-refreshes every 10 seconds</small></p>
</body>
</html>`))

// Handler serves the dashboard for m.
func Handler(m *monitor.Monitor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, m.Snapshot()); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	})
}
