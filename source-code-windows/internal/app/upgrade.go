package app

// The only HTTP code in the native app is a one-time client for stopping an
// older browser-based controller during upgrade. It serves no pages or API.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func retireLegacy(root string) (bool, error) {
	b, e := os.ReadFile(filepath.Join(root, "running.json"))
	if e != nil {
		return false, nil
	}
	u, e := url.Parse(string(b))
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" {
		return false, nil
	}
	token := strings.Trim(u.Path, "/")
	if !regexp.MustCompile(`^[0-9a-f]{48}$`).MatchString(token) {
		return false, nil
	}
	client := http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, action string) (*http.Response, error) {
		r, _ := http.NewRequest(method, "http://"+u.Host+"/api/"+action, bytes.NewBufferString("{}"))
		r.Header.Set("X-GeoCheats-Token", token)
		r.Header.Set("Content-Type", "application/json")
		return client.Do(r)
	}
	response, e := call("GET", "state")
	if e != nil {
		return false, nil
	}
	var state struct{ Name, Version string }
	e = json.NewDecoder(io.LimitReader(response.Body, 128*1024)).Decode(&state)
	_ = response.Body.Close()
	if e != nil || response.StatusCode != 200 || state.Name != "GeoCheats" {
		return false, nil
	}
	response, e = call("POST", "quit")
	if e != nil {
		return false, e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return false, fmt.Errorf("close the previous GeoCheats version before starting this build")
	}
	return true, nil
}
