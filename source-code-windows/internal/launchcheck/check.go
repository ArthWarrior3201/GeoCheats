// Package launchcheck provides an isolated app startup check for native packaging CI.
package launchcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func ReportPath(args []string) string {
	for i, arg := range args {
		if arg == "--launch-check" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func Run(server *http.Server, base, token, page, report, name string) {
	result := map[string]any{"ok": false, "name": name}
	defer func() {
		b, _ := json.MarshalIndent(result, "", "  ")
		_ = os.WriteFile(report, b, 0600)
		_ = server.Shutdown(context.Background())
	}()
	client := http.Client{Timeout: 3 * time.Second}
	request, _ := http.NewRequest("GET", base+"/api/state", nil)
	request.Header.Set("X-GeoCheats-Token", token)
	var response *http.Response
	var err error
	for n := 0; n < 50; n++ {
		response, err = client.Do(request)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		result["error"] = err.Error()
		return
	}
	defer response.Body.Close()
	var state struct {
		Name string `json:"name"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&state) != nil || state.Name != name {
		result["error"] = "authenticated state check failed"
		return
	}
	for _, resource := range []string{page, page + "icon.png"} {
		r, e := client.Get(resource)
		if e != nil {
			result["error"] = e.Error()
			return
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
		_ = r.Body.Close()
		if e != nil || r.StatusCode != 200 || len(b) == 0 {
			result["error"] = fmt.Sprintf("resource check failed: %s", resource)
			return
		}
		if resource == page && !strings.Contains(string(b), name) {
			result["error"] = "app page did not contain its name"
			return
		}
	}
	result["ok"] = true
	result["checks"] = []string{"app startup", "authenticated state", "embedded controls", "embedded app icon"}
}
