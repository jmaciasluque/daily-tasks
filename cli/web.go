package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"daily-tasks/internal"
)

//go:embed webui
var embeddedWebUI embed.FS

type webStateResponse struct {
	Action         string        `json:"action,omitempty"`
	Data           internal.Data `json:"data"`
	DataPath       string        `json:"data_path"`
	Backend        string        `json:"backend,omitempty"`
	Message        string        `json:"message,omitempty"`
	SyncConfigured bool          `json:"sync_configured"`
	Version        string        `json:"version"`
}

type webStatsResponse struct {
	Stats internal.StatsSummary `json:"stats"`
}

type webErrorResponse struct {
	Error string `json:"error"`
}

type webServer struct {
	dataPath   string
	configPath string
	staticFS   fs.FS
	loginFlows map[string]internal.LoginFlowV2Session
	mu         sync.Mutex
}

func serveWebApp(listenAddr string, shouldOpen bool) error {
	path, err := internal.DefaultDataPath()
	if err != nil {
		return err
	}
	configPath, err := internal.DefaultConfigPath()
	if err != nil {
		return err
	}

	staticFS, err := fs.Sub(embeddedWebUI, "webui")
	if err != nil {
		return fmt.Errorf("web assets are unavailable: %w", err)
	}

	srv := &webServer{
		dataPath:   path,
		configPath: configPath,
		staticFS:   staticFS,
		loginFlows: map[string]internal.LoginFlowV2Session{},
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	url := listenerURL(listener)
	fmt.Printf("Daily Tasks web app: %s\n", url)
	fmt.Printf("Local data file: %s\n", path)
	if cfg, _, err := internal.LoadEffectiveAppConfig(); err == nil && internal.IsBackendConfigured(cfg) {
		fmt.Printf("Backend: %s\n", cfg.Backend)
	} else {
		fmt.Println("Backend: not configured yet. Open the web app to choose local-only or connect Nextcloud.")
	}

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if shouldOpen {
		go openBrowser(url)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		fmt.Printf("\nStopping web server (%s)...\n", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func listenerURL(listener net.Listener) string {
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return "http://127.0.0.1"
	}
	host := addr.IP.String()
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, addr.Port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/data", s.handleData)
	mux.HandleFunc("/api/sync", s.handleSync)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/setup/local", s.handleSetupLocal)
	mux.HandleFunc("/api/setup/nextcloud/start", s.handleSetupNextcloudStart)
	mux.HandleFunc("/api/setup/nextcloud/poll", s.handleSetupNextcloudPoll)
	mux.Handle("/", s.handleSPA())
	return mux
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if !s.hasConfiguredBackend() {
		writeJSON(w, http.StatusConflict, webErrorResponse{Error: "backend setup is required"})
		return
	}

	s.mu.Lock()
	state, err := s.currentStateLocked("", "loaded")
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *webServer) handleData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w, http.MethodPut)
		return
	}
	if !s.hasConfiguredBackend() {
		writeJSON(w, http.StatusConflict, webErrorResponse{Error: "backend setup is required"})
		return
	}

	var data internal.Data
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{Error: "invalid JSON body"})
		return
	}

	s.mu.Lock()
	state, err := s.saveDataLocked(data)
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *webServer) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.hasConfiguredBackend() {
		writeJSON(w, http.StatusConflict, webErrorResponse{Error: "backend setup is required"})
		return
	}

	settings, err := internal.LoadWebDAVSettings()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{
			Error: "Nextcloud is not configured for this installation yet.",
		})
		return
	}

	s.mu.Lock()
	data, err := s.loadDataLocked()
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}

	result := internal.SyncWithRemote(settings, data)
	if result.Action == "error" {
		state := s.stateResponse(result.Data, result.Message, result.Action)
		s.mu.Unlock()
		writeJSON(w, http.StatusBadGateway, state)
		return
	}

	before := internal.CloneData(data)
	if err := internal.SaveDataWithHistory(s.dataPath, before, internal.NormalizeData(result.Data)); err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}

	state, err := s.currentStateLocked(result.Message, result.Action)
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *webServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if !s.hasConfiguredBackend() {
		writeJSON(w, http.StatusConflict, webErrorResponse{Error: "backend setup is required"})
		return
	}

	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))

	s.mu.Lock()
	data, err := s.loadDataLocked()
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	stats, err := internal.BuildStats(s.dataPath, data, from, to)
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, webStatsResponse{Stats: stats})
}

func (s *webServer) handleSetupLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	s.mu.Lock()
	err := internal.SaveAppConfig(s.configPath, internal.AppConfig{Backend: internal.BackendLocal})
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "configured"})
}

func (s *webServer) handleSetupNextcloudStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		ServerURL string `json:"server_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{Error: "invalid JSON body"})
		return
	}

	session, err := internal.StartLoginFlowV2(payload.ServerURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{Error: err.Error()})
		return
	}

	s.mu.Lock()
	sessionID := randomSessionID()
	s.loginFlows[sessionID] = session
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"login_url":  session.LoginURL,
	})
}

func (s *webServer) handleSetupNextcloudPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{Error: "session is required"})
		return
	}

	s.mu.Lock()
	session, ok := s.loginFlows[sessionID]
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, webErrorResponse{Error: "login session not found"})
		return
	}

	result, complete, err := internal.PollLoginFlowV2(session)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, webErrorResponse{Error: err.Error()})
		return
	}
	if !complete {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}

	s.mu.Lock()
	delete(s.loginFlows, sessionID)
	err = internal.SaveAppConfig(s.configPath, internal.AppConfigFromLogin(result))
	s.mu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webErrorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (s *webServer) handleSPA() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodHead)
			return
		}
		if !s.hasConfiguredBackend() {
			s.writeSetupPage(w)
			return
		}

		requestPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requestPath == "." || requestPath == "" {
			requestPath = "index.html"
		}

		if _, err := fs.Stat(s.staticFS, requestPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		cloned := r.Clone(r.Context())
		cloned.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, cloned)
	})
}

func (s *webServer) loadDataLocked() (internal.Data, error) {
	data, err := internal.LoadData(s.dataPath)
	if err != nil {
		return internal.Data{}, err
	}
	before := internal.CloneData(data)
	if internal.ResetIfNewDay(&data) {
		if err := internal.SaveDataWithHistory(s.dataPath, before, data); err != nil {
			return internal.Data{}, err
		}
		data, err = internal.LoadData(s.dataPath)
		if err != nil {
			return internal.Data{}, err
		}
	}
	return data, nil
}

func (s *webServer) currentStateLocked(message, action string) (webStateResponse, error) {
	data, err := s.loadDataLocked()
	if err != nil {
		return webStateResponse{}, err
	}
	return s.stateResponse(data, message, action), nil
}

func (s *webServer) saveDataLocked(data internal.Data) (webStateResponse, error) {
	current, err := s.loadDataLocked()
	if err != nil {
		return webStateResponse{}, err
	}

	data = internal.NormalizeData(data)
	internal.ResetIfNewDay(&data)
	if err := internal.SaveDataWithHistory(s.dataPath, current, data); err != nil {
		return webStateResponse{}, err
	}
	return s.currentStateLocked("Saved locally.", "saved")
}

func (s *webServer) stateResponse(data internal.Data, message, action string) webStateResponse {
	backend := ""
	if cfg, _, err := internal.LoadEffectiveAppConfig(); err == nil {
		backend = string(cfg.Backend)
	}
	return webStateResponse{
		Action:         action,
		Data:           internal.NormalizeData(data),
		DataPath:       s.dataPath,
		Backend:        backend,
		Message:        message,
		SyncConfigured: internal.HasWebDAVConfig(),
		Version:        internal.Version,
	}
}

func (s *webServer) hasConfiguredBackend() bool {
	cfg, _, err := internal.LoadEffectiveAppConfig()
	return err == nil && internal.IsBackendConfigured(cfg)
}

func (s *webServer) writeSetupPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Daily Tasks Setup</title>
  <style>
    :root { color-scheme: light; --bg:#f4f1e8; --panel:#fffaf1; --text:#1f1d18; --muted:#6e675a; --border:#d8cfbe; --accent:#e2c15b; }
    * { box-sizing:border-box; }
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background: radial-gradient(circle at top, #fff6d8 0%, var(--bg) 55%); color:var(--text); min-height:100vh; display:flex; align-items:center; justify-content:center; padding:24px; }
    .card { width:min(560px, 100%); background:var(--panel); border:1px solid var(--border); border-radius:24px; padding:28px; box-shadow: 0 18px 60px rgba(31,29,24,.08); }
    h1 { margin:0 0 8px; font-size:32px; }
    p { margin:0 0 12px; color:var(--muted); line-height:1.5; }
    .stack { display:grid; gap:12px; margin-top:18px; }
    input, button { width:100%; border-radius:14px; padding:14px 16px; font:inherit; }
    input { border:1px solid var(--border); background:#fff; color:var(--text); }
    button { border:1px solid var(--border); background:#fff; color:var(--text); cursor:pointer; }
    button.primary { background:var(--accent); border-color:#cba83b; font-weight:700; }
    .status { min-height:24px; margin-top:14px; color:var(--muted); }
    .divider { height:1px; background:var(--border); margin:18px 0; }
  </style>
</head>
<body>
  <main class="card">
    <h1>Choose a Backend</h1>
    <p>Daily Tasks now blocks first use until you choose how this installation should store and sync data.</p>
    <div class="stack">
      <button id="localButton">Use Local Only</button>
    </div>
    <div class="divider"></div>
    <p>Or connect Nextcloud once and let Daily Tasks generate a dedicated app password for this installation.</p>
    <div class="stack">
      <input id="serverUrl" type="url" placeholder="https://cloud.example.com">
      <button id="nextcloudButton" class="primary">Connect Nextcloud</button>
    </div>
    <p id="status" class="status"></p>
  </main>
  <script>
    const statusEl = document.getElementById('status');
    const serverUrlEl = document.getElementById('serverUrl');
    const localButton = document.getElementById('localButton');
    const nextcloudButton = document.getElementById('nextcloudButton');

    const setStatus = (text) => { statusEl.textContent = text; };

    localButton.addEventListener('click', async () => {
      setStatus('Saving local-only backend...');
      const res = await fetch('/api/setup/local', { method: 'POST' });
      if (!res.ok) {
        const payload = await res.json().catch(() => ({}));
        setStatus(payload.error || 'Could not save backend selection.');
        return;
      }
      window.location.reload();
    });

    nextcloudButton.addEventListener('click', async () => {
      const serverUrl = serverUrlEl.value.trim();
      if (!serverUrl) {
        setStatus('Enter your Nextcloud server URL first.');
        return;
      }

      setStatus('Starting Nextcloud login...');
      const res = await fetch('/api/setup/nextcloud/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_url: serverUrl }),
      });
      const payload = await res.json().catch(() => ({}));
      if (!res.ok) {
        setStatus(payload.error || 'Could not start Nextcloud login.');
        return;
      }

      window.open(payload.login_url, '_blank', 'noopener,noreferrer');
      setStatus('Finish the login in Nextcloud. This page will refresh once the connection is ready.');

      const poll = async () => {
        const pollRes = await fetch('/api/setup/nextcloud/poll?session=' + encodeURIComponent(payload.session_id));
        const pollPayload = await pollRes.json().catch(() => ({}));
        if (pollRes.status === 202) {
          window.setTimeout(poll, 2000);
          return;
        }
        if (!pollRes.ok) {
          setStatus(pollPayload.error || 'Nextcloud login failed.');
          return;
        }
        window.location.reload();
      };

      poll();
    });
  </script>
</body>
</html>`)
}

func randomSessionID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeJSON(w, http.StatusMethodNotAllowed, webErrorResponse{Error: "method not allowed"})
}
