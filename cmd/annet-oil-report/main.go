// Command annet-oil-report is a tiny standalone web service that exposes the
// result.json produced by `annet-oil check --collect-commands`:
//
//	GET /             -> HTML dashboard (stats + per-device table)
//	GET /result.json  -> the raw result.json file
//	GET /raw/<name>   -> a device's raw command output (from --output-dir)
//	GET /healthz      -> "ok"
//
// It reads result.json fresh on every request so it always reflects the latest
// sweep. It has no dependency on the running annet-oil API.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"annet-oil/internal/check"
)

var (
	resultPath = flag.String("result", "result.json", "Path to the result.json produced by 'annet-oil check --collect-commands'")
	outputDir  = flag.String("output-dir", "", "Directory with per-device raw command output (served under /raw/)")
	addr       = flag.String("addr", ":8182", "Listen address")
)

func main() {
	flag.Parse()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/result.json", serveResultJSON)
	http.HandleFunc("/raw/", serveRaw)
	http.HandleFunc("/", serveDashboard)

	log.Printf("annet-oil-report serving %s on %s (raw dir: %q)", *resultPath, *addr, *outputDir)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func loadReport() (*check.BatchReport, error) {
	b, err := os.ReadFile(*resultPath)
	if err != nil {
		return nil, err
	}
	var rep check.BatchReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

func serveResultJSON(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(*resultPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func serveRaw(w http.ResponseWriter, r *http.Request) {
	if *outputDir == "" {
		http.Error(w, "raw output not configured", http.StatusNotFound)
		return
	}
	name := filepath.Base(r.URL.Path[len("/raw/"):]) // Base() strips any path traversal
	if name == "" || name == "." || name == "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	full := filepath.Join(*outputDir, name)
	if filepath.Dir(full) != filepath.Clean(*outputDir) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, full)
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rep, err := loadReport()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h1>annet-oil report</h1><p>No result yet (%s): %v</p>", *resultPath, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, dashboardView(rep)); err != nil {
		log.Printf("template error: %v", err)
	}
}

type view struct {
	*check.BatchReport
	GeneratedAtStr string
	HasRaw         bool
	CommandsRun    bool
}

func dashboardView(rep *check.BatchReport) view {
	return view{
		BatchReport:    rep,
		GeneratedAtStr: rep.GeneratedAt.Format(time.RFC3339),
		HasRaw:         *outputDir != "",
		CommandsRun:    rep.CommandsOK+rep.CommandsFailed+rep.CommandsSkipped > 0,
	}
}

var dashboardTmpl = template.Must(template.New("dash").Funcs(template.FuncMap{
	"badge": func(ok bool) template.HTML {
		if ok {
			return template.HTML(`<span class="b ok">ok</span>`)
		}
		return template.HTML(`<span class="b bad">fail</span>`)
	},
	"cmdbadge": func(c *check.CommandResult) template.HTML {
		if c == nil {
			return template.HTML(`<span class="b skip">-</span>`)
		}
		switch c.Status {
		case check.CommandsOK:
			return template.HTML(`<span class="b ok">ok</span>`)
		case check.CommandsFailed:
			return template.HTML(`<span class="b bad">fail</span>`)
		default:
			return template.HTML(`<span class="b skip">skip</span>`)
		}
	},
	"loginbadge": func(s string) template.HTML {
		switch s {
		case check.LoginOK:
			return template.HTML(`<span class="b ok">ok</span>`)
		case check.LoginFailed:
			return template.HTML(`<span class="b bad">fail</span>`)
		default:
			return template.HTML(`<span class="b skip">skip</span>`)
		}
	},
	"errstr": func(e *check.CheckError) string {
		if e == nil {
			return ""
		}
		return e.Type + ": " + e.Message
	},
}).Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta http-equiv="refresh" content="30">
<title>annet-oil health report</title>
<style>
 body{font:14px/1.4 system-ui,sans-serif;margin:1.5rem;color:#1a1a1a;background:#fafafa}
 h1{font-size:1.3rem;margin:0 0 .3rem}
 .meta{color:#666;margin-bottom:1rem}
 .cards{display:flex;flex-wrap:wrap;gap:.6rem;margin-bottom:1rem}
 .card{background:#fff;border:1px solid #e2e2e2;border-radius:8px;padding:.6rem .9rem;min-width:110px}
 .card .n{font-size:1.5rem;font-weight:700}
 .card .l{color:#666;font-size:.8rem}
 table{border-collapse:collapse;width:100%;background:#fff;border:1px solid #e2e2e2;border-radius:8px;overflow:hidden}
 th,td{padding:.4rem .6rem;text-align:left;border-bottom:1px solid #eee;font-size:.85rem}
 th{background:#f3f3f3;position:sticky;top:0;cursor:pointer}
 tr:hover{background:#f9f9f9}
 .b{padding:.1rem .45rem;border-radius:10px;font-size:.75rem;font-weight:600}
 .ok{background:#d7f5dd;color:#12692b}
 .bad{background:#fbdcdc;color:#9b1c1c}
 .skip{background:#eee;color:#666}
 input{padding:.35rem .5rem;margin-bottom:.6rem;width:260px;border:1px solid #ccc;border-radius:6px}
 a{color:#2557d6;text-decoration:none}
 @media (prefers-color-scheme: dark){
  body{background:#161616;color:#e6e6e6}.card,table{background:#1f1f1f;border-color:#333}
  th{background:#262626}tr:hover{background:#242424}th,td{border-color:#2c2c2c}
  .skip{background:#333;color:#aaa}.meta{color:#999}.card .l{color:#999}input{background:#1f1f1f;color:#eee;border-color:#444}
 }
</style></head><body>
<h1>annet-oil health report</h1>
<div class="meta">Generated {{.GeneratedAtStr}} &middot; {{.DurationMs}}ms &middot; concurrency {{.Concurrency}} &middot;
 <a href="/result.json">result.json</a> &middot; auto-refresh 30s</div>
<div class="cards">
 <div class="card"><div class="n">{{.Total}}</div><div class="l">total</div></div>
 <div class="card"><div class="n">{{.Reachable}}</div><div class="l">network ok</div></div>
 <div class="card"><div class="n">{{.Unreachable}}</div><div class="l">unreachable</div></div>
 <div class="card"><div class="n">{{.LoginOK}}</div><div class="l">login ok</div></div>
 <div class="card"><div class="n">{{.LoginFailed}}</div><div class="l">login fail</div></div>
{{if .CommandsRun}}
 <div class="card"><div class="n">{{.CommandsOK}}</div><div class="l">commands ok</div></div>
 <div class="card"><div class="n">{{.CommandsFailed}}</div><div class="l">commands fail</div></div>
{{end}}
</div>
<input id="f" placeholder="filter (hostname, vendor, error...)" onkeyup="flt()">
<table id="t"><thead><tr>
 <th>hostname</th><th>ip</th><th>vendor</th><th>proto</th><th>network</th><th>login</th>
{{if .CommandsRun}}<th>commands</th>{{end}}<th>error</th>
</tr></thead><tbody>
{{range .Results}}
<tr>
 <td>{{if $.HasRaw}}<a href="/raw/{{.Hostname}}.txt">{{.Hostname}}</a>{{else}}{{.Hostname}}{{end}}</td>
 <td>{{.IP}}</td><td>{{.Vendor}}</td><td>{{.Protocol}}</td>
 <td>{{badge .Reachable}}</td>
 <td>{{loginbadge .Login}}</td>
{{if $.CommandsRun}} <td>{{cmdbadge .Commands}}</td>{{end}}
 <td>{{errstr .Error}}{{if .Commands}}{{if .Commands.Error}} {{.Commands.Error}}{{end}}{{end}}</td>
</tr>
{{end}}
</tbody></table>
<script>
function flt(){var q=document.getElementById('f').value.toLowerCase();
 document.querySelectorAll('#t tbody tr').forEach(function(r){
  r.style.display=r.innerText.toLowerCase().indexOf(q)>-1?'':'none';});}
</script>
</body></html>`))
