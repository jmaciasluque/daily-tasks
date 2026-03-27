package main

import (
	"context"
	"embed"
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
	Message        string        `json:"message,omitempty"`
	SyncConfigured bool          `json:"sync_configured"`
	Version        string        `json:"version"`
}

type webErrorResponse struct {
	Error string `json:"error"`
}

type webServer struct {
	dataPath string
	staticFS fs.FS
	mu       sync.Mutex
}

func serveWebApp(listenAddr string, shouldOpen bool) error {
	path, err := internal.DefaultDataPath()
	if err != nil {
		return err
	}

	staticFS, err := fs.Sub(embeddedWebUI, "webui")
	if err != nil {
		return fmt.Errorf("web assets are unavailable: %w", err)
	}

	srv := &webServer{
		dataPath: path,
		staticFS: staticFS,
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	url := listenerURL(listener)
	fmt.Printf("Daily Tasks web app: %s\n", url)
	fmt.Printf("Local data file: %s\n", path)
	if internal.HasWebDAVConfig() {
		fmt.Println("Nextcloud sync: configured from DAILY_TASKS_WEBDAV_*")
	} else {
		fmt.Println("Nextcloud sync: not configured. Set DAILY_TASKS_WEBDAV_URL, DAILY_TASKS_WEBDAV_USER, DAILY_TASKS_WEBDAV_PASS and restart.")
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
	mux.Handle("/", s.handleSPA())
	return mux
}

func (s *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
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

	settings, err := internal.LoadWebDAVSettings()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, webErrorResponse{
			Error: "Nextcloud sync is not configured. Set DAILY_TASKS_WEBDAV_URL, DAILY_TASKS_WEBDAV_USER, DAILY_TASKS_WEBDAV_PASS and restart the web server.",
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

	if err := internal.SaveData(s.dataPath, internal.NormalizeData(result.Data)); err != nil {
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
	if internal.ResetIfNewDay(&data) {
		if err := internal.SaveData(s.dataPath, data); err != nil {
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
	data = internal.NormalizeData(data)
	internal.ResetIfNewDay(&data)
	if err := internal.SaveData(s.dataPath, data); err != nil {
		return webStateResponse{}, err
	}
	return s.currentStateLocked("Saved locally.", "saved")
}

func (s *webServer) stateResponse(data internal.Data, message, action string) webStateResponse {
	return webStateResponse{
		Action:         action,
		Data:           internal.NormalizeData(data),
		DataPath:       s.dataPath,
		Message:        message,
		SyncConfigured: internal.HasWebDAVConfig(),
		Version:        internal.Version,
	}
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
