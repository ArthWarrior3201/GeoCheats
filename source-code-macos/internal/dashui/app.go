package dashui

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"geocheats/internal/core"
	"geocheats/internal/launchcheck"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

//go:embed ui.html dash.png
var assets embed.FS

type request struct {
	Game   string          `json:"game"`
	Recipe json.RawMessage `json:"recipe"`
}

func gamePath(path string) (string, error) {
	path = strings.Trim(strings.TrimSpace(path), "\"")
	if strings.HasSuffix(strings.ToLower(path), ".app") {
		entries, e := os.ReadDir(filepath.Join(path, "Contents", "MacOS"))
		if e != nil {
			return "", e
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "geometry") {
				return filepath.Abs(filepath.Join(path, "Contents", "MacOS", entry.Name()))
			}
		}
		return "", errors.New("no Geometry Dash executable found in the app bundle")
	}
	if path == "" {
		return "", errors.New("enter your installed game path")
	}
	return filepath.Abs(path)
}

func perform(action string, r request) (any, error) {
	path, e := gamePath(r.Game)
	if e != nil {
		return nil, e
	}
	im, e := core.Inspect(path)
	if e != nil {
		return nil, e
	}
	if action == "inspect" {
		return im, nil
	}
	var p core.Package
	switch action {
	case "mac-profile":
		p, e = core.MacInvincibility(path)
	case "build":
		if e = json.Unmarshal(r.Recipe, &p); e != nil {
			return nil, e
		}
		if p.Target == "AUTO" {
			p.Target = im.SHA256
		}
		if p.Format == "AUTO" {
			p.Format = im.Format
		}
		if p.Arch == "AUTO" {
			p.Arch = im.Arch
		}
	default:
		return nil, errors.New("unknown action")
	}
	if e != nil {
		return nil, e
	}
	if e = core.VerifyFile(p, path); e != nil {
		return nil, e
	}
	return map[string]any{"package": p, "filename": regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(p.ID, "_") + ".dash", "message": "Exact file hash and bytes verified. Gameplay still requires testing."}, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if e := cmd.Start(); e != nil {
		return e
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func Run() {
	random := make([]byte, 24)
	if _, e := rand.Read(random); e != nil {
		fmt.Println(e)
		return
	}
	token := hex.EncodeToString(random)
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		fmt.Println(e)
		return
	}
	base := "http://" + listener.Addr().String()
	page := base + "/" + token + "/"
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listener.Addr().String() {
			http.Error(w, "invalid host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		if r.Method == "GET" && r.URL.Path == "/"+token+"/" {
			b, _ := assets.ReadFile("ui.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(strings.ReplaceAll(string(b), "__TOKEN__", token)))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/"+token+"/icon.png" {
			b, _ := assets.ReadFile("dash.png")
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(b)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("X-GeoCheats-Token") != token {
			http.NotFound(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != base {
			http.Error(w, "cross-origin request denied", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		action := strings.TrimPrefix(r.URL.Path, "/api/")
		if action == "state" && r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "Dash", "version": "1.2.0"})
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var req request
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); e != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		if action == "quit" {
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Dash closed."})
			go func() { time.Sleep(100 * time.Millisecond); _ = server.Shutdown(context.Background()) }()
			return
		}
		result, e := perform(action, req)
		if e != nil {
			w.WriteHeader(409)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
	report := launchcheck.ReportPath(os.Args[1:])
	if report != "" {
		go launchcheck.Run(server, base, token, page, report, "Dash")
	} else if !(len(os.Args) > 1 && os.Args[1] == "--background") {
		if e = openBrowser(page); e != nil {
			fmt.Println("Open", page)
		}
	}
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	defer close(done)
	go func() {
		select {
		case <-signals:
			_ = server.Shutdown(context.Background())
		case <-done:
		}
	}()
	fmt.Println("Dash:", page)
	if e = server.Serve(listener); e != nil && e != http.ErrServerClosed {
		fmt.Println(e)
	}
}
