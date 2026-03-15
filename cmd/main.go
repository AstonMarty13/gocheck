package main

import (
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

const checkInterval = 10 * time.Second

type SiteStatus struct {
	URL         string
	Up          bool
	StatusCode  int
	ResponseMS  int64
	LastChecked time.Time
}

type StatusData struct {
	mu    sync.Mutex
	Sites map[string]*SiteStatus
}

var status = StatusData{Sites: make(map[string]*SiteStatus)}

func main() {
	links, err := readSites("sites.txt")
	if err != nil {
		fmt.Println("Erreur lecture sites.txt:", err)
		return
	}

	c := make(chan string)
	for _, link := range links {
		status.mu.Lock()
		status.Sites[link] = &SiteStatus{URL: link}
		status.mu.Unlock()
		go checkLink(link, c)
	}
	go func() {
		for l := range c {
			go func(link string) {
				time.Sleep(checkInterval)
				checkLink(link, c)
			}(l)
		}
	}()

	http.HandleFunc("/", handleHome)
	fmt.Println("Serveur démarré sur http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Erreur serveur:", err)
	}
}

func readSites(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func checkLink(link string, c chan string) {
	start := time.Now()
	resp, err := http.Get(link)
	elapsed := time.Since(start).Milliseconds()

	status.mu.Lock()
	s := status.Sites[link]
	s.LastChecked = time.Now()
	s.ResponseMS = elapsed
	if err != nil {
		s.Up = false
		s.StatusCode = 0
	} else {
		resp.Body.Close()
		s.Up = resp.StatusCode < 400
		s.StatusCode = resp.StatusCode
	}
	status.mu.Unlock()

	fmt.Printf("[%s] %s — %dms\n", time.Now().Format("15:04:05"), link, elapsed)
	c <- link
}

var tmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.Format("15:04:05")
	},
}).Parse(`<!DOCTYPE html>
<html lang="fr">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="10">
  <title>GoCheck Dashboard</title>
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
  <h1>GoCheck Dashboard</h1>
  <table>
    <thead>
      <tr><th>Site</th><th>Statut</th><th>Code HTTP</th><th>Temps de réponse</th><th>Dernier check</th></tr>
    </thead>
    <tbody>
    {{range .}}
      <tr>
        <td><a href="{{.URL}}" target="_blank">{{.URL}}</a></td>
        <td>{{if .Up}}<span class="up">✅ UP</span>{{else}}<span class="down">❌ DOWN</span>{{end}}</td>
        <td>{{if .StatusCode}}{{.StatusCode}}{{else}}—{{end}}</td>
        <td>{{if .LastChecked.IsZero}}—{{else}}{{.ResponseMS}} ms{{end}}</td>
        <td><small>{{formatTime .LastChecked}}</small></td>
      </tr>
    {{end}}
    </tbody>
  </table>
  <p><small>Actualisation automatique toutes les 10 secondes</small></p>
</body>
</html>`))

func handleHome(w http.ResponseWriter, r *http.Request) {
	status.mu.Lock()
	sites := make([]*SiteStatus, 0, len(status.Sites))
	for _, s := range status.Sites {
		sites = append(sites, s)
	}
	status.mu.Unlock()

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].URL < sites[j].URL
	})

	if err := tmpl.Execute(w, sites); err != nil {
		http.Error(w, "Erreur template", http.StatusInternalServerError)
	}
}
