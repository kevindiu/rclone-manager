package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
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

	svc := service.New(store, runEngine, sched, hub, client, cfg.Defaults, logger)
	authSvc := service.NewAuthService(store, 24*time.Hour, logger)
	fakeSPA := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><html><body>SPA OK</body></html>")},
	}
	srv := NewServer(cfg, svc, authSvc, client, runEngine, hub, fakeSPA, "test-1.0", logger)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts, dir
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func setupAuthenticatedClient(t *testing.T, ts *httptest.Server) *http.Client {
	t.Helper()
	c := newCookieClient(t)
	resp, err := c.Post(
		ts.URL+"/api/auth/setup",
		"application/json",
		strings.NewReader(`{"username":"admin","password":"SuperSecret123!"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/auth/setup = %d, want 201", resp.StatusCode)
	}
	return c
}

func TestAPIAuthMandatoryFlow(t *testing.T) {
	_, ts, _ := setupTestServer(t)

	// Before setup: /api/auth/status shows setup_required=true, authenticated=false.
	stResp, err := http.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	var st service.AuthStatus
	_ = json.NewDecoder(stResp.Body).Decode(&st)
	_ = stResp.Body.Close()
	if stResp.StatusCode != http.StatusOK || !st.SetupRequired || st.Authenticated {
		t.Fatalf("unexpected initial auth status: code=%d status=%+v", stResp.StatusCode, st)
	}

	// SPA & health remain public.
	for _, pub := range []string{"/", "/api/health"} {
		r, err := http.Get(ts.URL + pub)
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("public route %s = %d, want 200", pub, r.StatusCode)
		}
	}

	// Protected routes MUST return 401 when unauthenticated.
	protectedGets := []string{"/api/jobs", "/api/remotes", "/api/settings", "/api/events", "/api/system/info"}
	for _, p := range protectedGets {
		r, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated GET %s = %d, want 401", p, r.StatusCode)
		}
	}
	cpUnauth, err := http.Post(ts.URL+"/api/auth/change-password", "application/json", strings.NewReader(`{"current_password":"a","new_password":"NewPassword123!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = cpUnauth.Body.Close()
	if cpUnauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated change-password = %d, want 401", cpUnauth.StatusCode)
	}

	// Setup with weak password fails 400.
	badSetup, err := http.Post(ts.URL+"/api/auth/setup", "application/json", strings.NewReader(`{"username":"admin","password":"123"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = badSetup.Body.Close()
	if badSetup.StatusCode != http.StatusBadRequest {
		t.Fatalf("weak setup password = %d, want 400", badSetup.StatusCode)
	}

	// Valid setup succeeds 201 and sets HttpOnly session cookie.
	c1 := newCookieClient(t)
	okSetup, err := c1.Post(ts.URL+"/api/auth/setup", "application/json", strings.NewReader(`{"username":"admin","password":"InitialPassword123!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = okSetup.Body.Close()
	if okSetup.StatusCode != http.StatusCreated {
		t.Fatalf("valid setup = %d, want 201", okSetup.StatusCode)
	}
	var foundHttpOnly bool
	for _, ck := range okSetup.Cookies() {
		if ck.Name == sessionCookieName && ck.HttpOnly {
			foundHttpOnly = true
		}
	}
	if !foundHttpOnly {
		t.Fatal("expected HttpOnly session cookie on setup")
	}

	// Second setup attempt must be rejected (403).
	dupSetup, err := http.Post(ts.URL+"/api/auth/setup", "application/json", strings.NewReader(`{"username":"admin2","password":"InitialPassword123!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = dupSetup.Body.Close()
	if dupSetup.StatusCode != http.StatusForbidden {
		t.Fatalf("duplicate setup = %d, want 403", dupSetup.StatusCode)
	}

	// Authenticated status returns user.
	authedStResp, err := c1.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	var authedSt service.AuthStatus
	_ = json.NewDecoder(authedStResp.Body).Decode(&authedSt)
	_ = authedStResp.Body.Close()
	if !authedSt.Authenticated || authedSt.SetupRequired || authedSt.User == nil || authedSt.User.Username != "admin" {
		t.Fatalf("unexpected authenticated status: %+v", authedSt)
	}

	// Second session via Login (wrong password rejected, correct accepted).
	c2 := newCookieClient(t)
	badLogin, err := c2.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = badLogin.Body.Close()
	if badLogin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, want 401", badLogin.StatusCode)
	}
	goodLogin, err := c2.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"InitialPassword123!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = goodLogin.Body.Close()
	if goodLogin.StatusCode != http.StatusOK {
		t.Fatalf("good login = %d, want 200", goodLogin.StatusCode)
	}

	// Change password on c1 with wrong current fails 401.
	badCP, err := c1.Post(ts.URL+"/api/auth/change-password", "application/json", strings.NewReader(`{"current_password":"nope","new_password":"UpdatedPassword456!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = badCP.Body.Close()
	if badCP.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad change-password = %d, want 401", badCP.StatusCode)
	}

	// Valid change password keeps c1 valid and revokes c2.
	goodCP, err := c1.Post(ts.URL+"/api/auth/change-password", "application/json", strings.NewReader(`{"current_password":"InitialPassword123!","new_password":"UpdatedPassword456!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = goodCP.Body.Close()
	if goodCP.StatusCode != http.StatusOK {
		t.Fatalf("good change-password = %d, want 200", goodCP.StatusCode)
	}

	c1Check, err := c1.Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	_ = c1Check.Body.Close()
	if c1Check.StatusCode != http.StatusOK {
		t.Fatalf("expected c1 session to remain valid, got %d", c1Check.StatusCode)
	}
	c2Check, err := c2.Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	_ = c2Check.Body.Close()
	if c2Check.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected c2 session revoked after password change, got %d", c2Check.StatusCode)
	}

	// Logout clears c1 session.
	loResp, err := c1.Post(ts.URL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = loResp.Body.Close()
	if loResp.StatusCode != http.StatusOK {
		t.Fatalf("logout = %d, want 200", loResp.StatusCode)
	}
	afterLo, err := c1.Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	_ = afterLo.Body.Close()
	if afterLo.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", afterLo.StatusCode)
	}
}

func TestAPISecurityHeadersAndNoWildcardCORS(t *testing.T) {
	_, ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header by default, got %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", got)
	}

	// Cross-site state-changing browser request must be blocked by CSRF protection.
	csrfReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/jobs", strings.NewReader(`{}`))
	csrfReq.Header.Set("Origin", "https://evil.example.com")
	csrfReq.Header.Set("Sec-Fetch-Site", "cross-site")
	csrfResp, err := http.DefaultClient.Do(csrfReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = csrfResp.Body.Close()
	if csrfResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-site POST, got %d", csrfResp.StatusCode)
	}
}

func TestAPIHealthSettingsAndExportImport(t *testing.T) {
	_, ts, _ := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	// Health
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
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
	putResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT settings = %d", putResp.StatusCode)
	}

	// Export YAML & re-import
	expResp, err := client.Get(ts.URL + "/api/config/export?format=yaml")
	if err != nil {
		t.Fatal(err)
	}
	var yamlBuf bytes.Buffer
	_, _ = yamlBuf.ReadFrom(expResp.Body)
	_ = expResp.Body.Close()
	if expResp.StatusCode != http.StatusOK || !strings.Contains(yamlBuf.String(), "Asia/Tokyo") {
		t.Fatalf("unexpected YAML export: status=%d body=%s", expResp.StatusCode, yamlBuf.String())
	}

	impResp, err := client.Post(ts.URL+"/api/config/import?mode=merge", "application/yaml", bytes.NewReader(yamlBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	_ = impResp.Body.Close()
	if impResp.StatusCode != http.StatusOK {
		t.Fatalf("POST import YAML = %d", impResp.StatusCode)
	}
}

func TestAPIRemotesBrowseAndRestore(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	localRoot := filepath.Join(dir, "remote-root")
	if err := os.MkdirAll(filepath.Join(localRoot, "_versions", "2026-11-26"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localRoot, "notes.txt"), []byte("current-notes"), 0o644); err != nil {
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
	r1, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	_ = r1.Body.Close()
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
	r2, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatal(err)
	}
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("create crypt status = %d", r2.StatusCode)
	}

	// List remotes must mask crypt password
	listResp, err := client.Get(ts.URL + "/api/remotes")
	if err != nil {
		t.Fatal(err)
	}
	var remotes []rclone.RemoteInfo
	_ = json.NewDecoder(listResp.Body).Decode(&remotes)
	_ = listResp.Body.Close()
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}
	for _, rem := range remotes {
		if rem.Name == "mycrypt" && rem.Config["password"] != rclone.MaskedSecret {
			t.Fatalf("expected masked crypt password, got %q", rem.Config["password"])
		}
	}

	// Test remote connectivity
	testResp, err := client.Post(ts.URL+"/api/remotes/myalias/test", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var testRes rclone.RemoteTestResult
	_ = json.NewDecoder(testResp.Body).Decode(&testRes)
	_ = testResp.Body.Close()
	if !testRes.OK {
		t.Fatalf("expected test remote OK, got %+v", testRes)
	}

	// Browse & download
	browseResp, err := client.Get(ts.URL + "/api/browse?remote=myalias&path=_versions/2026-11-26")
	if err != nil {
		t.Fatal(err)
	}
	var browseData struct {
		Entries []rclone.BrowseEntry `json:"entries"`
	}
	_ = json.NewDecoder(browseResp.Body).Decode(&browseData)
	_ = browseResp.Body.Close()
	if len(browseData.Entries) != 1 || browseData.Entries[0].Name != "notes.txt" {
		t.Fatalf("unexpected browse entries: %+v", browseData.Entries)
	}

	dlResp, err := client.Get(ts.URL + "/api/browse/download?remote=myalias&path=_versions/2026-11-26/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	var dlBuf bytes.Buffer
	_, _ = dlBuf.ReadFrom(dlResp.Body)
	_ = dlResp.Body.Close()
	if dlBuf.String() != "hello-restore" {
		t.Fatalf("unexpected downloaded content: %q", dlBuf.String())
	}

	// Discover versions of notes.txt
	verResp, err := client.Get(ts.URL + "/api/browse/versions?remote=myalias&path=notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	var verPayload struct {
		Versions []rclone.FileVersion `json:"versions"`
	}
	_ = json.NewDecoder(verResp.Body).Decode(&verPayload)
	_ = verResp.Body.Close()
	if verResp.StatusCode != http.StatusOK || len(verPayload.Versions) != 1 || verPayload.Versions[0].Date != "2026-11-26" {
		t.Fatalf("unexpected versions response: status=%d versions=%+v", verResp.StatusCode, verPayload.Versions)
	}

	// Async Restore (202 Accepted) to myalias:restored/notes.txt
	restoreBody, _ := json.Marshal(rclone.RestoreRequest{
		SourceRemote: "myalias",
		SourcePath:   "_versions/2026-11-26/notes.txt",
		TargetPath:   "myalias:restored/notes.txt",
		IsDir:        false,
	})
	restResp, err := client.Post(ts.URL+"/api/browse/restore", "application/json", bytes.NewReader(restoreBody))
	if err != nil {
		t.Fatal(err)
	}
	var restRun domain.JobRun
	_ = json.NewDecoder(restResp.Body).Decode(&restRun)
	_ = restResp.Body.Close()
	if restResp.StatusCode != http.StatusAccepted || restRun.ID == "" {
		t.Fatalf("restore status=%d run=%+v", restResp.StatusCode, restRun)
	}

	waitSuccess := func(runID string) {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			gr, err := client.Get(ts.URL + "/api/runs/" + runID)
			if err == nil {
				var got domain.JobRun
				_ = json.NewDecoder(gr.Body).Decode(&got)
				_ = gr.Body.Close()
				if got.Status == "success" {
					return
				}
				if got.Status == "failed" || got.Status == "cancelled" {
					t.Fatalf("restore run %s ended with %s: %s", runID, got.Status, got.ErrorMessage)
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for restore run %s", runID)
	}
	waitSuccess(restRun.ID)

	restoredBytes, err := os.ReadFile(filepath.Join(localRoot, "restored", "notes.txt"))
	if err != nil || string(restoredBytes) != "hello-restore" {
		t.Fatalf("restored file mismatch: err=%v content=%q", err, string(restoredBytes))
	}

	// Verify conflict_mode="skip" preserves modified destination file
	if err := os.WriteFile(filepath.Join(localRoot, "restored", "notes.txt"), []byte("keep-local"), 0o644); err != nil {
		t.Fatal(err)
	}
	skipBody, _ := json.Marshal(rclone.RestoreRequest{
		SourceRemote: "myalias",
		SourcePath:   "_versions/2026-11-26/notes.txt",
		TargetPath:   "myalias:restored/notes.txt",
		ConflictMode: "skip",
	})
	skipResp, err := client.Post(ts.URL+"/api/browse/restore", "application/json", bytes.NewReader(skipBody))
	if err != nil {
		t.Fatal(err)
	}
	var skipRun domain.JobRun
	_ = json.NewDecoder(skipResp.Body).Decode(&skipRun)
	_ = skipResp.Body.Close()
	waitSuccess(skipRun.ID)

	keptBytes, err := os.ReadFile(filepath.Join(localRoot, "restored", "notes.txt"))
	if err != nil || string(keptBytes) != "keep-local" {
		t.Fatalf("expected skip mode to preserve local file, got %q err=%v", string(keptBytes), err)
	}

	// Browse file management: mkdir, multipart upload, size, rename, delete + root delete guard
	mkdirBody, _ := json.Marshal(map[string]string{"remote": "myalias", "path": "managed"})
	mkResp, err := client.Post(ts.URL+"/api/browse/mkdir", "application/json", bytes.NewReader(mkdirBody))
	if err != nil {
		t.Fatal(err)
	}
	_ = mkResp.Body.Close()
	if mkResp.StatusCode != http.StatusCreated {
		t.Fatalf("mkdir status = %d", mkResp.StatusCode)
	}

	var uploadBuf bytes.Buffer
	mw := multipart.NewWriter(&uploadBuf)
	fw, err := mw.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("uploaded-bytes"))
	_ = mw.Close()

	upReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/browse/upload?remote=myalias&path=managed", &uploadBuf)
	if err != nil {
		t.Fatal(err)
	}
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upResp, err := client.Do(upReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = upResp.Body.Close()
	if upResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", upResp.StatusCode)
	}

	szResp, err := client.Get(ts.URL + "/api/browse/size?remote=myalias&path=managed")
	if err != nil {
		t.Fatal(err)
	}
	var szRes struct {
		Count int64 `json:"count"`
		Bytes int64 `json:"bytes"`
	}
	_ = json.NewDecoder(szResp.Body).Decode(&szRes)
	_ = szResp.Body.Close()
	if szRes.Count != 1 || szRes.Bytes != int64(len("uploaded-bytes")) {
		t.Fatalf("unexpected DirSize result: %+v", szRes)
	}

	renBody, _ := json.Marshal(map[string]string{
		"remote":   "myalias",
		"path":     "managed/hello.txt",
		"new_name": "renamed.txt",
	})
	renResp, err := client.Post(ts.URL+"/api/browse/rename", "application/json", bytes.NewReader(renBody))
	if err != nil {
		t.Fatal(err)
	}
	_ = renResp.Body.Close()
	if renResp.StatusCode != http.StatusOK {
		t.Fatalf("rename status = %d", renResp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(localRoot, "managed", "renamed.txt")); err != nil {
		t.Fatalf("expected renamed file on disk: %v", err)
	}

	delFileReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/browse/entry?remote=myalias&path=managed/renamed.txt&is_dir=0", nil)
	delFileResp, err := client.Do(delFileReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delFileResp.Body.Close()
	if delFileResp.StatusCode != http.StatusOK {
		t.Fatalf("delete file status = %d", delFileResp.StatusCode)
	}

	delDirReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/browse/entry?remote=myalias&path=managed&is_dir=1", nil)
	delDirResp, err := client.Do(delDirReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delDirResp.Body.Close()
	if delDirResp.StatusCode != http.StatusOK {
		t.Fatalf("delete dir status = %d", delDirResp.StatusCode)
	}

	// Root deletion must always be rejected
	delRootReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/browse/entry?remote=myalias&path=&is_dir=1", nil)
	delRootResp, err := client.Do(delRootReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delRootResp.Body.Close()
	if delRootResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 deleting root, got %d", delRootResp.StatusCode)
	}
}

func TestAPIJobsRunsAndSSE(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)
	src := filepath.Join(dir, "job-src")
	dst := filepath.Join(dir, "job-dst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(dst, 0o755)
	_ = os.WriteFile(filepath.Join(src, "file.txt"), []byte("payload"), 0o644)

	// Connect SSE client and verify snapshot event arrives
	sseResp, err := client.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sseResp.Body.Close() }()
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
	cResp, err := client.Post(ts.URL+"/api/jobs", "application/json", bytes.NewReader(jobPayload))
	if err != nil {
		t.Fatal(err)
	}
	var created domain.Job
	_ = json.NewDecoder(cResp.Body).Decode(&created)
	_ = cResp.Body.Close()
	if cResp.StatusCode != http.StatusCreated || created.ID == "" {
		t.Fatalf("create job failed: status=%d job=%+v", cResp.StatusCode, created)
	}

	// Run job
	runResp, err := client.Post(ts.URL+"/api/jobs/"+created.ID+"/run", "application/json", strings.NewReader(`{"dry_run":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var run domain.JobRun
	_ = json.NewDecoder(runResp.Body).Decode(&run)
	_ = runResp.Body.Close()
	if runResp.StatusCode != http.StatusAccepted {
		t.Fatalf("run job status = %d", runResp.StatusCode)
	}

	// Poll run completion
	deadline := time.Now().Add(10 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		gr, err := client.Get(ts.URL + "/api/runs/" + run.ID)
		if err == nil {
			var got domain.JobRun
			_ = json.NewDecoder(gr.Body).Decode(&got)
			_ = gr.Body.Close()
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
	logsResp, err := client.Get(ts.URL + "/api/runs/" + run.ID + "/logs")
	if err != nil {
		t.Fatal(err)
	}
	var logsPayload struct {
		Raw string `json:"raw"`
	}
	_ = json.NewDecoder(logsResp.Body).Decode(&logsPayload)
	_ = logsResp.Body.Close()
	if logsPayload.Raw == "" {
		t.Fatal("expected non-empty run logs")
	}

	// Clone job
	cloneResp, err := client.Post(ts.URL+"/api/jobs/"+created.ID+"/clone", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = cloneResp.Body.Close()
	if cloneResp.StatusCode != http.StatusCreated {
		t.Fatalf("clone job status = %d", cloneResp.StatusCode)
	}
}

func TestAPIRemoteDeleteConflictGuard(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	remotePayload, _ := json.Marshal(map[string]any{
		"name":   "guardremote",
		"type":   "alias",
		"params": map[string]string{"remote": dir},
	})
	remResp, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(remotePayload))
	if err != nil {
		t.Fatal(err)
	}
	_ = remResp.Body.Close()
	if remResp.StatusCode != http.StatusCreated {
		t.Fatalf("create remote status = %d", remResp.StatusCode)
	}

	jobPayload, _ := json.Marshal(domain.Job{
		Name:        "Ref Job",
		Enabled:     false,
		Operation:   "copy",
		Source:      dir,
		Destination: "guardremote:docs",
	})
	jResp, err := client.Post(ts.URL+"/api/jobs", "application/json", bytes.NewReader(jobPayload))
	if err != nil {
		t.Fatal(err)
	}
	var created domain.Job
	_ = json.NewDecoder(jResp.Body).Decode(&created)
	_ = jResp.Body.Close()
	if jResp.StatusCode != http.StatusCreated {
		t.Fatalf("create job status = %d", jResp.StatusCode)
	}

	// Deleting referenced remote must return 409 Conflict.
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/remotes/guardremote", nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delResp.Body.Close()
	if delResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict deleting referenced remote, got %d", delResp.StatusCode)
	}

	// After removing the job, deleting the remote succeeds.
	delJobReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/jobs/"+created.ID, nil)
	delJobResp, err := client.Do(delJobReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delJobResp.Body.Close()
	if delJobResp.StatusCode != http.StatusOK {
		t.Fatalf("delete job status = %d", delJobResp.StatusCode)
	}

	delReq2, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/remotes/guardremote", nil)
	delResp2, err := client.Do(delReq2)
	if err != nil {
		t.Fatal(err)
	}
	_ = delResp2.Body.Close()
	if delResp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK deleting unreferenced remote, got %d", delResp2.StatusCode)
	}
}

func TestAPIAuthRateLimit429(t *testing.T) {
	_, ts, _ := setupTestServer(t)
	_ = setupAuthenticatedClient(t, ts)

	// Send 10 empty-password login failures (skips expensive PBKDF2 work while testing IP limiter).
	for i := 0; i < 10; i++ {
		resp, err := http.Post(
			ts.URL+"/api/auth/login",
			"application/json",
			strings.NewReader(`{"username":"admin","password":""}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401", i+1, resp.StatusCode)
		}
	}

	// 11th attempt must hit 429 Too Many Requests with Retry-After header.
	throttled, err := http.Post(
		ts.URL+"/api/auth/login",
		"application/json",
		strings.NewReader(`{"username":"admin","password":""}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = throttled.Body.Close()
	if throttled.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th attempt status = %d, want 429", throttled.StatusCode)
	}
	if throttled.Header.Get("Retry-After") == "" {
		t.Fatal("expected non-empty Retry-After header on 429 response")
	}
}

func TestAPICORSAndTrustedOriginCSRF(t *testing.T) {
	srv, ts, _ := setupTestServer(t)
	srv.cfg.CORSOrigins = []string{"https://ui.example.com"}
	if err := srv.csrf.AddTrustedOrigin("https://ui.example.com"); err != nil {
		t.Fatal(err)
	}

	// Allowed CORS origin gets echo + Allow-Credentials: true.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "https://ui.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://ui.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want https://ui.example.com", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}

	// Trusted origin passes CrossOriginProtection on cross-site POST.
	okReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/logout", nil)
	okReq.Header.Set("Origin", "https://ui.example.com")
	okReq.Header.Set("Sec-Fetch-Site", "cross-site")
	okResp, err := http.DefaultClient.Do(okReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = okResp.Body.Close()
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for trusted cross-origin POST, got %d", okResp.StatusCode)
	}

	// Untrusted origin fails CrossOriginProtection with 403 Forbidden.
	evilReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/logout", nil)
	evilReq.Header.Set("Origin", "https://evil.example.com")
	evilReq.Header.Set("Sec-Fetch-Site", "cross-site")
	evilResp, err := http.DefaultClient.Do(evilReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = evilResp.Body.Close()
	if evilResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for untrusted cross-origin POST, got %d", evilResp.StatusCode)
	}
}

func TestAPIMalformedJSONAndPathTraversal(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	remotePayload, _ := json.Marshal(map[string]any{
		"name":   "myalias",
		"type":   "alias",
		"params": map[string]string{"remote": dir},
	})
	remResp, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(remotePayload))
	if err != nil {
		t.Fatal(err)
	}
	_ = remResp.Body.Close()

	jobPayload, _ := json.Marshal(domain.Job{
		Name:        "JSON Check Job",
		Operation:   "copy",
		Source:      dir,
		Destination: "myalias:out",
	})
	jResp, err := client.Post(ts.URL+"/api/jobs", "application/json", bytes.NewReader(jobPayload))
	if err != nil {
		t.Fatal(err)
	}
	var created domain.Job
	_ = json.NewDecoder(jResp.Body).Decode(&created)
	_ = jResp.Body.Close()

	// Malformed JSON body on /run must return 400 Bad Request.
	badRunResp, err := client.Post(ts.URL+"/api/jobs/"+created.ID+"/run", "application/json", strings.NewReader("{bad-json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = badRunResp.Body.Close()
	if badRunResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed /run JSON, got %d", badRunResp.StatusCode)
	}

	// Path traversal on browse download must be rejected (404 from StatFile rejecting '..').
	travResp, err := client.Get(ts.URL + "/api/browse/download?remote=myalias&path=../secret")
	if err != nil {
		t.Fatal(err)
	}
	_ = travResp.Body.Close()
	if travResp.StatusCode == http.StatusOK {
		t.Fatal("expected traversal download request to fail, got 200 OK")
	}
}

func TestAPINewEndpointsAndInlinePreview(t *testing.T) {
	_, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	// Preview command endpoint
	prevBody := `{"job":{"name":"Prev","operation":"bisync","source":"/src","destination":"r2:dst","min_age":"2h"},"dry_run":true,"resync":true}`
	prevResp, err := client.Post(ts.URL+"/api/jobs/preview-command", "application/json", strings.NewReader(prevBody))
	if err != nil {
		t.Fatal(err)
	}
	var prevRes struct {
		Command string `json:"command"`
	}
	_ = json.NewDecoder(prevResp.Body).Decode(&prevRes)
	_ = prevResp.Body.Close()
	if prevResp.StatusCode != http.StatusOK || !strings.Contains(prevRes.Command, "bisync") || !strings.Contains(prevRes.Command, "--resync") {
		t.Fatalf("unexpected preview response: status=%d cmd=%q", prevResp.StatusCode, prevRes.Command)
	}

	// Daily run stats & clear finished runs
	statsResp, err := client.Get(ts.URL + "/api/runs/stats?days=7")
	if err != nil {
		t.Fatal(err)
	}
	var stats []domain.DailyRunStat
	_ = json.NewDecoder(statsResp.Body).Decode(&stats)
	_ = statsResp.Body.Close()
	if statsResp.StatusCode != http.StatusOK || len(stats) != 7 {
		t.Fatalf("unexpected run stats: status=%d len=%d", statsResp.StatusCode, len(stats))
	}

	clrReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/runs?status=success", nil)
	clrResp, err := client.Do(clrReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = clrResp.Body.Close()
	if clrResp.StatusCode != http.StatusOK {
		t.Fatalf("clear runs status = %d", clrResp.StatusCode)
	}

	// Active sessions & revoke others
	c2 := newCookieClient(t)
	loginResp, err := c2.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"SuperSecret123!"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("c2 login = %d", loginResp.StatusCode)
	}

	sessResp, err := client.Get(ts.URL + "/api/auth/sessions")
	if err != nil {
		t.Fatal(err)
	}
	var sessions []service.SessionInfo
	_ = json.NewDecoder(sessResp.Body).Decode(&sessions)
	_ = sessResp.Body.Close()
	if sessResp.StatusCode != http.StatusOK || len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got status=%d sessions=%+v", sessResp.StatusCode, sessions)
	}

	revResp, err := client.Post(ts.URL+"/api/auth/sessions/revoke-others", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = revResp.Body.Close()
	if revResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke-others = %d", revResp.StatusCode)
	}
	c2Check, err := c2.Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	_ = c2Check.Body.Close()
	if c2Check.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked c2 session to get 401, got %d", c2Check.StatusCode)
	}

	// Inline safe Content-Type vs XSS block + bulk delete
	root := filepath.Join(dir, "preview-root")
	_ = os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hello"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "page.html"), []byte("<script>alert(1)</script>"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("a"), 0o644)

	remPayload, _ := json.Marshal(map[string]any{
		"name":   "prevremote",
		"type":   "alias",
		"params": map[string]string{"remote": root},
	})
	remResp, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(remPayload))
	if err != nil {
		t.Fatal(err)
	}
	_ = remResp.Body.Close()

	txtResp, err := client.Get(ts.URL + "/api/browse/download?remote=prevremote&path=readme.txt&inline=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = txtResp.Body.Close()
	if txtResp.StatusCode != http.StatusOK || !strings.HasPrefix(txtResp.Header.Get("Content-Type"), "text/plain") || !strings.HasPrefix(txtResp.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("unexpected .txt inline headers: status=%d ct=%q cd=%q", txtResp.StatusCode, txtResp.Header.Get("Content-Type"), txtResp.Header.Get("Content-Disposition"))
	}

	htmlResp, err := client.Get(ts.URL + "/api/browse/download?remote=prevremote&path=page.html&inline=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = htmlResp.Body.Close()
	if htmlResp.StatusCode != http.StatusOK || htmlResp.Header.Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(htmlResp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("html must NOT be served inline: status=%d ct=%q cd=%q", htmlResp.StatusCode, htmlResp.Header.Get("Content-Type"), htmlResp.Header.Get("Content-Disposition"))
	}

	bulkPayload := `{"remote":"prevremote","entries":[{"path":"readme.txt","is_dir":false},{"path":"sub","is_dir":true}]}`
	bulkResp, err := client.Post(ts.URL+"/api/browse/bulk-delete", "application/json", strings.NewReader(bulkPayload))
	if err != nil {
		t.Fatal(err)
	}
	var bulkRes struct {
		Deleted int   `json:"deleted"`
		Failed  []any `json:"failed"`
	}
	_ = json.NewDecoder(bulkResp.Body).Decode(&bulkRes)
	_ = bulkResp.Body.Close()
	if bulkResp.StatusCode != http.StatusOK || bulkRes.Deleted != 2 || len(bulkRes.Failed) != 0 {
		t.Fatalf("unexpected bulk-delete result: status=%d res=%+v", bulkResp.StatusCode, bulkRes)
	}

	// Saving Telegram bot token masks token in PUT & GET responses while retaining secret server-side.
	tgReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", strings.NewReader(`{"transfers":4,"checkers":8,"timezone":"UTC","max_concurrent_jobs":2,"webhook_type":"telegram","telegram_bot_token":"123456:REAL-BOT-SECRET","telegram_chat_id":"-100777"}`))
	tgReq.Header.Set("Content-Type", "application/json")
	tgResp, err := client.Do(tgReq)
	if err != nil {
		t.Fatal(err)
	}
	var savedSettings domain.GlobalSettings
	_ = json.NewDecoder(tgResp.Body).Decode(&savedSettings)
	_ = tgResp.Body.Close()
	if tgResp.StatusCode != http.StatusOK || savedSettings.TelegramBotToken != rclone.MaskedSecret {
		t.Fatalf("expected masked telegram_bot_token on PUT, got status=%d token=%q", tgResp.StatusCode, savedSettings.TelegramBotToken)
	}
	getSettingsResp, err := client.Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	var fetchedSettings domain.GlobalSettings
	_ = json.NewDecoder(getSettingsResp.Body).Decode(&fetchedSettings)
	_ = getSettingsResp.Body.Close()
	if fetchedSettings.TelegramBotToken != rclone.MaskedSecret || fetchedSettings.WebhookType != "telegram" || fetchedSettings.TelegramChatID != "-100777" {
		t.Fatalf("unexpected masked GET /api/settings: %+v", fetchedSettings)
	}

	// Test draft remote connectivity without persisting to rclone.conf
	draftDir := filepath.Join(dir, "draft-remote-dir")
	_ = os.MkdirAll(draftDir, 0o755)
	_ = os.WriteFile(filepath.Join(draftDir, "hello.txt"), []byte("hi"), 0o644)

	remListBeforeResp, err := client.Get(ts.URL + "/api/remotes")
	if err != nil {
		t.Fatal(err)
	}
	var remBefore []rclone.RemoteInfo
	_ = json.NewDecoder(remListBeforeResp.Body).Decode(&remBefore)
	_ = remListBeforeResp.Body.Close()

	draftTestBody, _ := json.Marshal(map[string]any{
		"name":   "unsaved-alias",
		"type":   "alias",
		"params": map[string]string{"remote": draftDir},
	})
	draftTestResp, err := client.Post(ts.URL+"/api/remotes/test-draft", "application/json", bytes.NewReader(draftTestBody))
	if err != nil {
		t.Fatal(err)
	}
	var draftTestRes rclone.RemoteTestResult
	_ = json.NewDecoder(draftTestResp.Body).Decode(&draftTestRes)
	_ = draftTestResp.Body.Close()
	if draftTestResp.StatusCode != http.StatusOK || !draftTestRes.OK {
		t.Fatalf("unexpected test-draft response: status=%d res=%+v", draftTestResp.StatusCode, draftTestRes)
	}

	remListAfterResp, err := client.Get(ts.URL + "/api/remotes")
	if err != nil {
		t.Fatal(err)
	}
	var remAfter []rclone.RemoteInfo
	_ = json.NewDecoder(remListAfterResp.Body).Decode(&remAfter)
	_ = remListAfterResp.Body.Close()
	if len(remAfter) != len(remBefore) {
		t.Fatalf("test-draft must not persist remote: before=%d after=%d", len(remBefore), len(remAfter))
	}

	// Test draft job dry run without creating job or writing destination files
	draftDst := filepath.Join(dir, "draft-job-dst")
	_ = os.MkdirAll(draftDst, 0o755)
	draftJobBody, _ := json.Marshal(domain.Job{
		Operation:   "copy",
		Source:      draftDir,
		Destination: draftDst,
	})
	draftJobResp, err := client.Post(ts.URL+"/api/jobs/dry-run", "application/json", bytes.NewReader(draftJobBody))
	if err != nil {
		t.Fatal(err)
	}
	var draftDryRes runner.DraftDryRunResult
	_ = json.NewDecoder(draftJobResp.Body).Decode(&draftDryRes)
	_ = draftJobResp.Body.Close()
	if draftJobResp.StatusCode != http.StatusOK || !draftDryRes.OK {
		t.Fatalf("unexpected draft dry-run response: status=%d res=%+v", draftJobResp.StatusCode, draftDryRes)
	}
	if _, statErr := os.Stat(filepath.Join(draftDst, "hello.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("draft dry-run must not copy files to destination, stat err=%v", statErr)
	}
}

func TestFullEncryptedBackupRoundTrip(t *testing.T) {
	srv, ts, dir := setupTestServer(t)
	client := setupAuthenticatedClient(t, ts)

	remoteDir := filepath.Join(dir, "backup-src")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Create a remote
	remBody, _ := json.Marshal(map[string]any{
		"name":   "bakremote",
		"type":   "alias",
		"params": map[string]string{"remote": remoteDir},
	})
	remResp, err := client.Post(ts.URL+"/api/remotes", "application/json", bytes.NewReader(remBody))
	if err != nil {
		t.Fatal(err)
	}
	_ = remResp.Body.Close()
	if remResp.StatusCode != http.StatusCreated {
		t.Fatalf("create remote status = %d", remResp.StatusCode)
	}

	// 2. Save settings with sensitive telegram_bot_token
	setBody := `{"transfers":6,"checkers":10,"bwlimit":"10M","retries":3,"low_level_retries":10,"timeout":"5m","contimeout":"1m","log_retention_days":30,"timezone":"UTC","max_concurrent_jobs":2,"webhook_type":"telegram","telegram_bot_token":"secret-tg-token-999","telegram_chat_id":"12345","webhook_on":"failure"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", strings.NewReader(setBody))
	req.Header.Set("Content-Type", "application/json")
	setResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = setResp.Body.Close()
	if setResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT settings status = %d", setResp.StatusCode)
	}

	// 3. Create a job referencing the remote
	jobBody, _ := json.Marshal(map[string]any{
		"name":        "BackupJob",
		"operation":   "copy",
		"source":      remoteDir,
		"destination": "bakremote:out",
		"enabled":     true,
	})
	jobResp, err := client.Post(ts.URL+"/api/jobs", "application/json", bytes.NewReader(jobBody))
	if err != nil {
		t.Fatal(err)
	}
	var createdJob domain.Job
	_ = json.NewDecoder(jobResp.Body).Decode(&createdJob)
	_ = jobResp.Body.Close()
	if jobResp.StatusCode != http.StatusCreated {
		t.Fatalf("create job status = %d", jobResp.StatusCode)
	}

	// 4. Short passphrase must fail with 400
	shortResp, err := client.Post(ts.URL+"/api/config/export-full", "application/json", strings.NewReader(`{"passphrase":"short"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = shortResp.Body.Close()
	if shortResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("short passphrase status = %d, want 400", shortResp.StatusCode)
	}

	// 5. Export full encrypted backup
	expResp, err := client.Post(ts.URL+"/api/config/export-full", "application/json", strings.NewReader(`{"passphrase":"StrongPassphrase!2026"}`))
	if err != nil {
		t.Fatal(err)
	}
	var encBuf bytes.Buffer
	_, _ = encBuf.ReadFrom(expResp.Body)
	_ = expResp.Body.Close()
	if expResp.StatusCode != http.StatusOK {
		t.Fatalf("export-full status = %d body=%s", expResp.StatusCode, encBuf.String())
	}
	if cd := expResp.Header.Get("Content-Disposition"); !strings.Contains(cd, ".rcmbak") {
		t.Fatalf("expected .rcmbak Content-Disposition, got %q", cd)
	}
	if strings.Contains(encBuf.String(), "bakremote") || strings.Contains(encBuf.String(), "secret-tg-token-999") {
		t.Fatal("encrypted export leaked raw remote name or telegram token")
	}

	// 6. Delete job and remote so state is clean before restore
	delJobReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/jobs/"+createdJob.ID, nil)
	delJobResp, err := client.Do(delJobReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delJobResp.Body.Close()
	if delJobResp.StatusCode != http.StatusOK {
		t.Fatalf("delete job status = %d", delJobResp.StatusCode)
	}

	delRemReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/remotes/bakremote", nil)
	delRemResp, err := client.Do(delRemReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = delRemResp.Body.Close()
	if delRemResp.StatusCode != http.StatusOK {
		t.Fatalf("delete remote status = %d", delRemResp.StatusCode)
	}

	// 7. Wrong passphrase restore must fail with 400
	badImpBody, _ := json.Marshal(map[string]string{
		"content":    encBuf.String(),
		"passphrase": "WrongPassphrase!2026",
		"mode":       "replace",
	})
	badImpResp, err := client.Post(ts.URL+"/api/config/import-full", "application/json", bytes.NewReader(badImpBody))
	if err != nil {
		t.Fatal(err)
	}
	_ = badImpResp.Body.Close()
	if badImpResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong passphrase import status = %d, want 400", badImpResp.StatusCode)
	}

	// 8. Correct passphrase restore restores remote (with 0600 rclone.conf) and job
	goodImpBody, _ := json.Marshal(map[string]string{
		"content":    encBuf.String(),
		"passphrase": "StrongPassphrase!2026",
		"mode":       "replace",
	})
	goodImpResp, err := client.Post(ts.URL+"/api/config/import-full", "application/json", bytes.NewReader(goodImpBody))
	if err != nil {
		t.Fatal(err)
	}
	var impRes struct {
		Status          string   `json:"status"`
		JobsImported    int      `json:"jobs_imported"`
		RemotesImported int      `json:"remotes_imported"`
		RemoteNames     []string `json:"remote_names"`
	}
	_ = json.NewDecoder(goodImpResp.Body).Decode(&impRes)
	_ = goodImpResp.Body.Close()
	if goodImpResp.StatusCode != http.StatusOK || impRes.JobsImported != 1 || impRes.RemotesImported != 1 {
		t.Fatalf("unexpected import-full response: status=%d res=%+v", goodImpResp.StatusCode, impRes)
	}

	info, err := os.Stat(srv.cfg.RcloneConfig)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("rclone.conf mode = %o, want 0600", perm)
	}

	listRemResp, err := client.Get(ts.URL + "/api/remotes")
	if err != nil {
		t.Fatal(err)
	}
	var rems []rclone.RemoteInfo
	_ = json.NewDecoder(listRemResp.Body).Decode(&rems)
	_ = listRemResp.Body.Close()
	if len(rems) != 1 || rems[0].Name != "bakremote" {
		t.Fatalf("expected restored remote bakremote, got %+v", rems)
	}

	listJobResp, err := client.Get(ts.URL + "/api/jobs")
	if err != nil {
		t.Fatal(err)
	}
	var jobs []domain.Job
	_ = json.NewDecoder(listJobResp.Body).Decode(&jobs)
	_ = listJobResp.Body.Close()
	if len(jobs) != 1 || jobs[0].Name != "BackupJob" {
		t.Fatalf("expected restored job BackupJob, got %+v", jobs)
	}
}
