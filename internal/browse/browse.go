// Package browse serves generated wiki docs via a built-in Go HTTP server.
// No Node.js or npm required.
package browse

import (
"bytes"
"encoding/json"
"fmt"
"html/template"
"net/http"
"os"
"os/exec"
"path/filepath"
"runtime"
"strings"

"github.com/yuin/goldmark"
"github.com/yuin/goldmark/extension"
"github.com/yuin/goldmark/parser"
ghtml "github.com/yuin/goldmark/renderer/html"
)

// ── data types ─────────────────────────────────────────────────────────────────

type pageInfo struct {
Title   string `json:"title"`
Slug    string `json:"slug"`
Section string `json:"section,omitempty"`
}

type wikiData struct {
Pages []pageInfo `json:"pages"`
}

type navCategory struct {
Label string
Items []pageInfo
}

type pageData struct {
ProjectName string
Title       string
Section     string
ActiveSlug  string
Categories  []navCategory
Content     template.HTML
StaticLinks bool
}

// ── public entry point ─────────────────────────────────────────────────────────

// Run starts the documentation server (or builds a static site if build=true).
// siteDir is intentionally unused (kept for CLI compat).
func Run(workDir, _ string, port int, openBrowser, build bool) error {
docsDir, isDraft, err := resolveDocsDir(workDir)
if err != nil {
return err
}

wiki, err := readWiki(docsDir)
if err != nil {
return err
}

if isDraft {
fmt.Println("⚠️  正在预览草稿（尚未提交的生成内容）")
}

projectName := filepath.Base(workDir)
cats := buildCategories(wiki.Pages)

md := goldmark.New(
goldmark.WithExtensions(extension.GFM, extension.Table, extension.Strikethrough, extension.TaskList),
goldmark.WithParserOptions(parser.WithAutoHeadingID()),
goldmark.WithRendererOptions(ghtml.WithUnsafe()),
)

tmpl, err := template.New("page").Parse(htmlTmpl)
if err != nil {
return fmt.Errorf("解析 HTML 模板失败: %w", err)
}

if build {
return buildStatic(workDir, docsDir, projectName, cats, wiki.Pages, md, tmpl)
}

return serve(docsDir, projectName, cats, wiki, md, tmpl, port, openBrowser)
}

// resolveDocsDir finds the best docs directory to serve.
// Priority:
//  1. .zread/wiki/versions/<current> — committed version (from WikiCurrentFile)
//  2. .zread/wiki/versions/<latest>  — most recent version if no current marker
//  3. .zread/wiki/drafts/            — fallback for in-progress generation
func resolveDocsDir(workDir string) (dir string, isDraft bool, err error) {
wikiDir := filepath.Join(workDir, ".zread", "wiki")
versionsDir := filepath.Join(wikiDir, "versions")
draftsDir := filepath.Join(wikiDir, "drafts")

// Try "current" pointer file first
currentFile := filepath.Join(wikiDir, "current")
if data, e := os.ReadFile(currentFile); e == nil {
versionID := strings.TrimSpace(string(data))
if versionID != "" {
vDir := filepath.Join(versionsDir, versionID)
if _, e2 := os.Stat(filepath.Join(vDir, "wiki.json")); e2 == nil {
return vDir, false, nil
}
}
}

// Scan versions/ for the latest directory
if entries, e := os.ReadDir(versionsDir); e == nil && len(entries) > 0 {
// Pick last entry (sorted by name; zread uses date-based IDs)
var latest string
for _, entry := range entries {
if entry.IsDir() {
vDir := filepath.Join(versionsDir, entry.Name())
if _, e2 := os.Stat(filepath.Join(vDir, "wiki.json")); e2 == nil {
latest = vDir
}
}
}
if latest != "" {
return latest, false, nil
}
}

// Fallback to drafts
if _, e := os.Stat(filepath.Join(draftsDir, "wiki.json")); e == nil {
return draftsDir, true, nil
}

return "", false, fmt.Errorf("找不到文档，请先运行 zread generate")
}

// ── HTTP server ────────────────────────────────────────────────────────────────

func serve(docsDir, projectName string, cats []navCategory, wiki *wikiData, md goldmark.Markdown, tmpl *template.Template, port int, openBrowser bool) error {
mux := http.NewServeMux()

mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
slug := strings.TrimPrefix(r.URL.Path, "/")
if slug == "" {
if len(wiki.Pages) > 0 {
http.Redirect(w, r, "/"+wiki.Pages[0].Slug, http.StatusFound)
} else {
http.NotFound(w, r)
}
return
}
renderPage(w, slug, docsDir, projectName, cats, md, tmpl, false)
})

addr := fmt.Sprintf(":%d", port)
fmt.Printf("🚀 文档服务已启动 → \033[36mhttp://localhost%s\033[0m\n", addr)
fmt.Printf("   共 %d 个页面 | 按 Ctrl+C 停止\n", len(wiki.Pages))

if openBrowser {
go openURL(fmt.Sprintf("http://localhost%s", addr))
}

return http.ListenAndServe(addr, mux)
}

func renderPage(w http.ResponseWriter, slug, docsDir, projectName string, cats []navCategory, md goldmark.Markdown, tmpl *template.Template, static bool) {
title, section := resolveTitle(slug, cats)

var content template.HTML
raw, err := os.ReadFile(filepath.Join(docsDir, slug+".md"))
if err == nil {
var buf bytes.Buffer
if err2 := md.Convert(raw, &buf); err2 == nil {
content = template.HTML(buf.String())
}
}

data := pageData{
ProjectName: projectName,
Title:       title,
Section:     section,
ActiveSlug:  slug,
Categories:  cats,
Content:     content,
StaticLinks: static,
}

w.Header().Set("Content-Type", "text/html; charset=utf-8")
if err := tmpl.Execute(w, data); err != nil {
http.Error(w, err.Error(), 500)
}
}

// ── static build ───────────────────────────────────────────────────────────────

func buildStatic(workDir, docsDir, projectName string, cats []navCategory, pages []pageInfo, md goldmark.Markdown, tmpl *template.Template) error {
buildDir := filepath.Join(workDir, ".zread", "build")
if err := os.MkdirAll(buildDir, 0o755); err != nil {
return err
}

fmt.Printf("🔨 构建静态站点 → %s\n", buildDir)
for i, p := range pages {
title, section := resolveTitle(p.Slug, cats)
var content template.HTML
raw, err := os.ReadFile(filepath.Join(docsDir, p.Slug+".md"))
if err == nil {
var buf bytes.Buffer
if err2 := md.Convert(raw, &buf); err2 == nil {
content = template.HTML(buf.String())
}
}
data := pageData{
ProjectName: projectName,
Title:       title,
Section:     section,
ActiveSlug:  p.Slug,
Categories:  cats,
Content:     content,
StaticLinks: true,
}
var buf bytes.Buffer
if err := tmpl.Execute(&buf, data); err != nil {
fmt.Printf("  ⚠ 跳过 %s: %v\n", p.Slug, err)
continue
}
dest := filepath.Join(buildDir, p.Slug+".html")
os.WriteFile(dest, buf.Bytes(), 0o644)
if i == 0 {
os.WriteFile(filepath.Join(buildDir, "index.html"), buf.Bytes(), 0o644)
}
fmt.Printf("  ✓ %s.html\n", p.Slug)
}
fmt.Printf("\n✅ 完成，共 %d 个页面\n输出: %s\n", len(pages), buildDir)
return nil
}

// ── helpers ────────────────────────────────────────────────────────────────────

func readWiki(docsDir string) (*wikiData, error) {
data, err := os.ReadFile(filepath.Join(docsDir, "wiki.json"))
if err != nil {
return nil, fmt.Errorf("找不到 wiki.json，请先运行 zread generate：%w", err)
}
var wiki wikiData
if err := json.Unmarshal(data, &wiki); err != nil {
return nil, fmt.Errorf("解析 wiki.json 失败：%w", err)
}
if len(wiki.Pages) == 0 {
return nil, fmt.Errorf("wiki 中没有页面，请先运行 zread generate")
}
return &wiki, nil
}

func buildCategories(pages []pageInfo) []navCategory {
catMap := map[string]*navCategory{}
order := []string{}
for _, p := range pages {
sec := p.Section
if sec == "" {
sec = "文档"
}
if _, ok := catMap[sec]; !ok {
order = append(order, sec)
catMap[sec] = &navCategory{Label: sec}
}
catMap[sec].Items = append(catMap[sec].Items, p)
}
out := make([]navCategory, 0, len(order))
for _, s := range order {
out = append(out, *catMap[s])
}
return out
}

func resolveTitle(slug string, cats []navCategory) (title, section string) {
for _, c := range cats {
for _, p := range c.Items {
if p.Slug == slug {
return p.Title, c.Label
}
}
}
return slug, ""
}

func openURL(url string) {
switch runtime.GOOS {
case "windows":
exec.Command("cmd", "/c", "start", url).Start()
case "darwin":
exec.Command("open", url).Start()
default:
exec.Command("xdg-open", url).Start()
}
}

// ── HTML template ──────────────────────────────────────────────────────────────

const htmlTmpl = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — {{.ProjectName}}</title>
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --sb-w:272px;
  --sb-bg:#16181d;
  --sb-border:rgba(255,255,255,0.07);
  --sb-text:#8b92a8;
  --sb-text-h:#ffffff;
  --sb-active:#82aaff;
  --sb-active-bg:rgba(130,170,255,0.12);
  --accent:#6366f1;
  --bg:#ffffff;
  --border:#e5e7eb;
  --text:#1f2937;
  --muted:#6b7280;
  --heading:#111827;
  --code-bg:#f3f4f6;
  --pre-bg:#1a1b26;
  --pre-text:#a9b1d6;
  --link:#2563eb;
  --radius:8px;
}
html,body{height:100%}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,"Noto Sans SC",sans-serif;
  background:var(--bg);color:var(--text);display:flex;overflow:hidden;height:100vh}
.sb{width:var(--sb-w);min-width:var(--sb-w);background:var(--sb-bg);
  display:flex;flex-direction:column;overflow-y:auto;height:100vh;
  border-right:1px solid var(--sb-border)}
.sb::-webkit-scrollbar{width:3px}
.sb::-webkit-scrollbar-thumb{background:rgba(255,255,255,0.08);border-radius:2px}
.sb-head{padding:1.25rem 1.125rem 1rem;border-bottom:1px solid var(--sb-border);flex-shrink:0}
.sb-project{font-size:0.9375rem;font-weight:700;color:var(--sb-text-h);
  letter-spacing:-0.01em;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.sb-badge{display:inline-flex;align-items:center;gap:4px;margin-top:5px;
  font-size:0.68rem;font-weight:600;letter-spacing:0.04em;text-transform:uppercase;
  color:var(--sb-active);background:rgba(130,170,255,0.14);
  padding:2px 8px;border-radius:999px}
.sb-nav{padding:0.5rem 0 1.5rem;flex:1}
.cat-label{padding:1rem 1.125rem 0.3rem;font-size:0.675rem;font-weight:700;
  letter-spacing:0.09em;text-transform:uppercase;color:#3d4466;user-select:none}
.nav-a{display:block;padding:0.32rem 1.125rem 0.32rem 1.375rem;
  color:var(--sb-text);text-decoration:none;font-size:0.8375rem;
  border-left:2px solid transparent;transition:all 0.13s;
  white-space:nowrap;overflow:hidden;text-overflow:ellipsis;line-height:1.55}
.nav-a:hover{color:var(--sb-text-h);background:rgba(255,255,255,0.04)}
.nav-a.on{color:var(--sb-active);border-left-color:var(--sb-active);
  background:var(--sb-active-bg);font-weight:500}
.main{flex:1;min-width:0;display:flex;flex-direction:column;overflow:hidden}
.topbar{background:rgba(255,255,255,0.94);backdrop-filter:blur(10px);
  border-bottom:1px solid var(--border);padding:0.6875rem 2rem;
  display:flex;align-items:center;gap:0.5rem;font-size:0.8rem;
  color:var(--muted);flex-shrink:0;z-index:10}
.topbar-sep{color:#d1d5db}
.topbar-cur{color:var(--text);font-weight:500}
.scroll{flex:1;overflow-y:auto}
.scroll::-webkit-scrollbar{width:5px}
.scroll::-webkit-scrollbar-thumb{background:#e5e7eb;border-radius:3px}
.content{max-width:860px;padding:2.25rem 2.75rem 4rem;margin:0 auto}
.content h1{font-size:1.8rem;font-weight:750;color:var(--heading);
  margin-bottom:0.625rem;letter-spacing:-0.025em;line-height:1.2}
.content h2{font-size:1.3rem;font-weight:650;color:var(--heading);
  margin:2rem 0 0.625rem;padding-bottom:0.4rem;border-bottom:1px solid var(--border)}
.content h3{font-size:1.075rem;font-weight:600;color:var(--heading);margin:1.5rem 0 0.4rem}
.content h4,.content h5{font-weight:600;color:var(--heading);margin:1.25rem 0 0.35rem}
.content p{line-height:1.85;margin-bottom:0.9rem;color:#374151}
.content ul,.content ol{padding-left:1.5rem;margin-bottom:0.9rem}
.content li{line-height:1.8;margin-bottom:0.2rem}
.content a{color:var(--link);text-decoration:none}
.content a:hover{text-decoration:underline}
.content strong{color:var(--heading);font-weight:600}
.content blockquote{border-left:3px solid var(--accent);padding:0.6rem 1rem;
  margin:1rem 0;background:#faf5ff;color:#5b21b6;border-radius:0 var(--radius) var(--radius) 0}
.content hr{border:none;border-top:1px solid var(--border);margin:2rem 0}
.content img{max-width:100%;border-radius:var(--radius);margin:0.5rem 0}
.content code{background:var(--code-bg);padding:.15em .42em;border-radius:5px;
  font-family:"JetBrains Mono","Fira Code",Consolas,monospace;font-size:.855em;color:#be185d}
.content pre{background:var(--pre-bg);color:var(--pre-text);padding:1.1rem 1.375rem;
  border-radius:var(--radius);overflow-x:auto;margin:0.75rem 0 1.25rem;
  font-size:.855rem;line-height:1.65;border:1px solid rgba(255,255,255,0.05)}
.content pre code{background:none;padding:0;color:inherit;font-size:inherit;border-radius:0}
.content table{width:100%;border-collapse:collapse;margin:0.75rem 0 1.25rem;font-size:.9rem}
.content th{background:#f9fafb;font-weight:600;text-align:left;
  padding:.55rem 1rem;border:1px solid var(--border)}
.content td{padding:.45rem 1rem;border:1px solid var(--border)}
.content tr:nth-child(even) td{background:#fafafa}
.mermaid-wrap{background:#fafafa;border:1px solid var(--border);
  border-radius:var(--radius);padding:1.25rem;margin:1rem 0;text-align:center;overflow-x:auto}
.empty{display:flex;flex-direction:column;align-items:center;justify-content:center;
  height:55vh;text-align:center;color:var(--muted)}
.empty-icon{font-size:3rem;margin-bottom:1rem}
.empty h2{font-size:1.1rem;color:var(--heading);margin-bottom:.5rem}
.empty code{background:var(--code-bg);padding:.15em .4em;border-radius:4px;font-size:.875em}
</style>
</head>
<body>
<aside class="sb">
  <div class="sb-head">
    <div class="sb-project">{{.ProjectName}}</div>
    <span class="sb-badge">📄 Wiki</span>
  </div>
  <nav class="sb-nav">
    {{- range .Categories}}
    <div class="cat-label">{{.Label}}</div>
    {{- range .Items}}
    {{- $slug := .Slug}}
    <a href="{{if $.StaticLinks}}{{$slug}}.html{{else}}/{{$slug}}{{end}}"
       class="nav-a{{if eq $.ActiveSlug $slug}} on{{end}}" title="{{.Title}}">{{.Title}}</a>
    {{- end}}
    {{- end}}
  </nav>
</aside>
<div class="main">
  <div class="topbar">
    <span>{{.ProjectName}}</span>
    {{- if .Section}}<span class="topbar-sep">›</span><span>{{.Section}}</span>{{end}}
    <span class="topbar-sep">›</span>
    <span class="topbar-cur">{{.Title}}</span>
  </div>
  <div class="scroll">
    <div class="content">
      {{- if .Content}}
        {{.Content}}
      {{- else}}
      <div class="empty">
        <div class="empty-icon">��</div>
        <h2>页面尚未生成</h2>
        <p>请先运行 <code>zread generate</code> 生成该页面内容。</p>
      </div>
      {{- end}}
    </div>
  </div>
</div>
<script>
document.querySelectorAll('pre > code.language-mermaid').forEach(function(el){
  var wrap=document.createElement('div');wrap.className='mermaid-wrap';
  var div=document.createElement('div');div.className='mermaid';div.textContent=el.textContent;
  wrap.appendChild(div);el.parentElement.replaceWith(wrap);
});
if(document.querySelector('.mermaid')){
  var s=document.createElement('script');s.src='https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js';
  s.onload=function(){mermaid.initialize({startOnLoad:true,theme:'default'});};document.head.appendChild(s);
}
(function(){
  var links=Array.from(document.querySelectorAll('.nav-a'));
  var cur=links.findIndex(function(a){return a.classList.contains('on');});
  document.addEventListener('keydown',function(e){
    if(e.target.tagName==='INPUT'||e.target.tagName==='TEXTAREA')return;
    if(e.key==='ArrowRight'&&cur<links.length-1)location.href=links[cur+1].href;
    if(e.key==='ArrowLeft'&&cur>0)location.href=links[cur-1].href;
  });
})();
</script>
</body>
</html>`
