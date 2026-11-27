package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kevindiu/rclone-manager/internal/config"
	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/events"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/runner"
	"github.com/kevindiu/rclone-manager/internal/scheduler"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func setupTestServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Host:         "127.0.0.1",
		Port:         "0",
		DataDir:      dir,
		DBPath:       filepath.Join(dir, "test.db"),
		RcloneConfig: filepath.Join(dir, "rclone.conf"),
		LogDir:       filepath.Join(dir, "logs"),
		RcloneBin:    "rclone",
		Defaults: domain.GlobalSettings{
			Transfers:         4,
			Checkers:          8,
			Retries:           3,
			LowLevelRetries:   10,
			Timeout:           "5m",
			ConTimeout:        "1m",
			LogRetentionDays:  30,
			Timezone:          "UTC",
			MaxConcurrentJobs: 2,
		},
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	logger := slog.Default()
	client := rclone.NewClient(cfg.RcloneBin, cfg.RcloneConfig)
	hub := events.NewHub()
	runEngine := runner.NewRunner(store, client, hub, cfg.LogDir, cfg.Defaults, logger)
	sched := scheduler.New(store, runEngine, cfg.Defaults, logger)
	if err := sched.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sched.Stop)

	svc := service.New(store, runEngine, sched, hub, cfg.Defaults, logger)
	fakeSPA := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><html><body>SPA OK</body></html>")},
	}
	srv := NewServer(cfg, svc, client, runEngine, hub, fakeSPA, "test-1.0", logger)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts, dir
}

func TestAPISecurityHeadersAndNoWildcardCORS(t *testing.T) {
	_, ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header by default, got %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", got)
	}
}

func TestAPIHealthSettingsAndExportImport(t *testing.T) {
	_, ts, _ := setupTestServer(t)

	// Health
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" {
		t.Fatalf("unexpected health: %+v", health)
	}

	// Update settings
	body := `{"transfers":8,"checkers":16,"bwlimit":"08:00,10M","retries":5,"low_level_retries":10,"timeout":"5m","contimeout":"1m","log_retention_days":14,"timezone":"Asia/Tokyo","max_concurrent_jobs":3}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT settings = %d", putResp.StatusCode)
	}

	// Export YAML & re-import
	expResp, err := http.Get(ts.URL + "/api/config/export?format=yaml")
	if err != nil {
		t.Fatal(err)
	}
	var yamlBuf bytes.Buffer
	_, _ = yamlBuf.ReadFrom(expResp.Body)
	expResp.Body.Close()
	if expResp.StatusCode != http.StatusOK || !strings.Contains(yamlBuf.String(), "Asia/Tokyo") {
		t.Fatalf("unexpected YAML export: status=%d body=%s", expResp.StatusCode, yamlBuf.String())
	}

	impResp, err := http.Post(ts.URL+"/api/config/import?mode=merge", "application/yaml", bytes.NewReader(yamlBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	impResp.Body.Close()
	if impResp.StatusCode != http.StatusOK {
		t.Fatalf("POST import YAML = %d", impResp.StatusCode)
	}
}

func TestAPIRemotesBrowseAndRestore(t *testing.T) {
	_, ts, dir := setupTestServer(t)

	localRoot := filepath.Join(dir, "remote-root")
	if err := os.MkdirAll(filepath.Join(localRoot, "_versions", "2026-11-26"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localRoot, "_versions", "2026-11-26", "notes.txt"), []byte("hello-restore"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create alias remote and crypt remote on top of it
	createAlias := map[string]any{
		"name": "myalias",
		"type": "alias",
		"params": map[string]string{
			"remote": localRoot,
		},
	}
	b1, _ := json.Marshal(createAlias)
	r1, err := http.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	r1.Body.Close()
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("create alias status = %d", r1.StatusCode)
	}

	createCrypt := map[string]any{
		"name": "mycrypt",
		"type": "crypt",
		"params": map[string]string{
			"remote":   filepath.Join(dir, "crypt-store"),
			"password": "very-secret-passphrase-1234",
		},
	}
	b2, _ := json.Marshal(createCrypt)
	r2, err := http.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("create crypt status = %d", r2.StatusCode)
	}

	// List remotes must mask crypt password
	listResp, err := http.Get(ts.URL + "/api/remotes")
	if err != nil {
		t.Fatal(err)
	}
	var remotes []rclone.RemoteInfo
	_ = json.NewDecoder(listResp.Body).Decode(&remotes)
	listResp.Body.Close()
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}
	for _, rem := range remotes {
		if rem.Name == "mycrypt" && rem.Config["password"] != rclone.MaskedSecret {
			t.Fatalf("expected masked crypt password, got %q", rem.Config["password"])
		}
	}

	// Test remote connectivity
	testResp, err := http.Post(ts.URL+"/api/remotes/myalias/test", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var testRes rclone.RemoteTestResult
	_ = json.NewDecoder(testResp.Body).Decode(&testRes)
	testResp.Body.Close()
	if !testRes.OK {
		t.Fatalf("expected test remote OK, got %+v", testRes)
	}

	// Browse & download
	browseResp, err := http.Get(ts.URL + "/api/browse?remote=myalias&path=_versions/2026-11-26")
	if err != nil {
		t.Fatal(err)
	}
	var browseData struct {
		Entries []rclone.BrowseEntry `json:"entries"`
	}
	_ = json.NewDecoder(browseResp.Body).Decode(&browseData)
	browseResp.Body.Close()
	if len(browseData.Entries) != 1 || browseData.Entries[0].Name != "notes.txt" {
		t.Fatalf("unexpected browse entries: %+v", browseData.Entries)
	}

	dlResp, err := http.Get(ts.URL + "/api/browse/download?remote=myalias&path=_versions/2026-11-26/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	var dlBuf bytes.Buffer
	_, _ = dlBuf.ReadFrom(dlResp.Body)
	dlResp.Body.Close()
	if dlBuf.String() != "hello-restore" {
		t.Fatalf("unexpected downloaded content: %q", dlBuf.String())
	}

	// Restore to myalias:restored/notes.txt
	restoreBody, _ := json.Marshal(rclone.RestoreRequest{
		SourceRemote: "myalias",
		SourcePath:   "_versions/2026-11-26/notes.txt",
		TargetPath:   "myalias:restored/notes.txt",
		IsDir:        false,
	})
	restResp, err := http.Post(ts.URL+"/api/browse/restore", "application/json", bytes.NewReader(restoreBody))
	if err != nil {
		t.Fatal(err)
	}
	restResp.Body.Close()
	if restResp.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d", restResp.StatusCode)
	}
	restoredBytes, err := os.ReadFile(filepath.Join(localRoot, "restored", "notes.txt"))
	if err != nil || string(restoredBytes) != "hello-restore" {
		t.Fatalf("restored file mismatch: err=%v content=%q", err, string(restoredBytes))
	}
}

func TestAPIJobsRunsAndSSE(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	src := filepath.Join(dir, "job-src")
	dst := filepath.Join(dir, "job-dst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(dst, 0o755)
	_ = os.WriteFile(filepath.Join(src, "file.txt"), []byte("payload"), 0o644)

	// Connect SSE client and verify snapshot event arrives
	sseResp, err := http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()
	reader := bufio.NewReader(sseResp.Body)
	line1, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(line1, "event: snapshot") {
		t.Fatalf("expected snapshot event, got %q err=%v", line1, err)
	}

	// Create job
	jobPayload, _ := json.Marshal(domain.Job{
		Name:        "API Job",
		Enabled:     true,
		Operation:   "copy",
		Source:      src,
		Destination: dst,
		Schedule:    "0 4 * * *",
	})
	cResp, err := http.Post(ts.URL+"/api/jobs", "application/json", bytes.NewReader(jobPayload))
	if err != nil {
		t.Fatal(err)
	}
	var created domain.Job
	_ = json.NewDecoder(cResp.Body).Decode(&created)
	cResp.Body.Close()
	if cResp.StatusCode != http.StatusCreated || created.ID == "" {
		t.Fatalf("create job failed: status=%d job=%+v", cResp.StatusCode, created)
	}

	// Run job
	runResp, err := http.Post(ts.URL+"/api/jobs/"+created.ID+"/run", "application/json", strings.NewReader(`{"dry_run":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var run domain.JobRun
	_ = json.NewDecoder(runResp.Body).Decode(&run)
	runResp.Body.Close()
	if runResp.StatusCode != http.StatusAccepted {
		t.Fatalf("run job status = %d", runResp.StatusCode)
	}

	// Poll run completion
	deadline := time.Now().Add(10 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		gr, err := http.Get(ts.URL + "/api/runs/" + run.ID)
		if err == nil {
			var got domain.JobRun
			_ = json.NewDecoder(gr.Body).Decode(&got)
			gr.Body.Close()
			if got.Status == "success" || got.Status == "failed" {
				finalStatus = got.Status
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if finalStatus != "success" {
		t.Fatalf("expected run success, got %q", finalStatus)
	}

	// Logs endpoint
	logsResp, err := http.Get(ts.URL + "/api/runs/" + run.ID + "/logs")
	if err != nil {
		t.Fatal(err)
	}
	var logsPayload struct {
		Raw string `json:"raw"`
	}
	_ = json.NewDecoder(logsResp.Body).Decode(&logsPayload)
	logsResp.Body.Close()
	if logsPayload.Raw == "" {
		t.Fatal("expected non-empty run logs")
	}

	// Clone job
	cloneResp, err := http.Post(ts.URL+"/api/jobs/"+created.ID+"/clone", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	cloneResp.Body.Close()
	if cloneResp.StatusCode != http.StatusCreated {
		t.Fatalf("clone job status = %d", cloneResp.StatusCode)
	}
}
