package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/meetingv2"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

type apiProvider struct{}

func (apiProvider) Transcribe(context.Context, meetingv2.TranscribeRequest) (meetingv2.TranscribeResult, error) {
	return meetingv2.TranscribeResult{Text: "ok"}, nil
}
func (apiProvider) Check(context.Context) error { return nil }

func setupMeetingV2Router(t *testing.T, scopes []string) (*gin.Engine, *meetingv2.Store, *meetingv2.Worker, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	store, err := meetingv2.NewStore(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := meetingv2.NewWorker(store, apiProvider{}, meetingv2.WorkerOptions{Concurrency: 1, MaxAttempts: 2, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Second, ProviderName: "test", Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("user", "alice")
		context.Set("scopes", scopes)
		context.Set("meeting_resource_scopes", scopes)
		context.Next()
	})
	NewMeetingV2Handler(store, worker, apiProvider{}, 100).Register(router.Group("/api/v1/meeting-sessions"))
	return router, store, worker, root
}

func setupMeetingV2RouterWithoutASR(t *testing.T, scopes []string) (*gin.Engine, *meetingv2.Store, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	store, err := meetingv2.NewStore(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("user", "alice")
		context.Set("scopes", scopes)
		context.Set("meeting_resource_scopes", scopes)
		context.Next()
	})
	NewMeetingV2Handler(store, nil, nil, 100, false).Register(router.Group("/api/v1/meeting-sessions"))
	return router, store, root
}

func TestMeetingV2ScopeIsFailClosed(t *testing.T) {
	router, _, _, _ := setupMeetingV2Router(t, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/meeting-sessions", bytes.NewBufferString(`{"title":"x"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestMeetingV2CreateWritesOwnerProjection(t *testing.T) {
	router, _, _, root := setupMeetingV2Router(t, []string{users.ScopeMeetingWrite})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/meeting-sessions", bytes.NewBufferString(`{"title":"meeting","metadata":{"source":"browser"}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Session meetingv2.Session `json:"session"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Session.Owner != "alice" {
		t.Fatalf("owner=%q", payload.Session.Owner)
	}
	metadataPath := filepath.Join(root, payload.Session.ID, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil || !bytes.Contains(data, []byte(`"owner": "alice"`)) {
		t.Fatalf("owner projection=%q err=%v", data, err)
	}
}

func TestMeetingV2RejectsUnknownCreateField(t *testing.T) {
	router, _, _, _ := setupMeetingV2Router(t, []string{users.ScopeMeetingWrite})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/meeting-sessions", bytes.NewBufferString(`{"title":"x","owner":"mallory"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMeetingV2UploadValidationAndIdempotency(t *testing.T) {
	router, store, worker, _ := setupMeetingV2Router(t, []string{users.ScopeMeetingWrite, users.ScopeMeetingRead})
	session, err := store.CreateOwnedSession("upload-session", "", "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState(session.ID, meetingv2.StateRecording); err != nil {
		t.Fatal(err)
	}
	makeRequest := func(index string, audio string) *http.Request {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/meeting-sessions/upload-session/chunks/"+index, bytes.NewBufferString(audio))
		request.Header.Set("Content-Type", "audio/webm")
		request.Header.Set("X-AIDG-Chunk-Duration-Ms", "3000")
		return request
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, makeRequest("0", "audio"))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, makeRequest("0", "audio"))
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, makeRequest("100", "audio"))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("large index status=%d", response.Code)
	}
	_ = worker
}

func TestMeetingV2ReadRequiresReadScope(t *testing.T) {
	router, store, _, _ := setupMeetingV2Router(t, []string{users.ScopeMeetingWrite})
	if _, err := store.CreateOwnedSession("read-session", "", "alice", nil); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/read-session", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestMeetingV2GlobalScopeDoesNotGrantCrossMeetingAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := meetingv2.NewStore(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateOwnedSession("bob-meeting", "", "bob", nil); err != nil {
		t.Fatal(err)
	}
	worker, _ := meetingv2.NewWorker(store, apiProvider{}, meetingv2.WorkerOptions{Concurrency: 1, MaxAttempts: 2, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Second, ProviderName: "test", Model: "test"})
	router := gin.New()
	router.Use(func(context *gin.Context) {
		context.Set("user", "alice")
		context.Set("scopes", []string{users.ScopeMeetingRead, users.ScopeMeetingWrite})
		context.Set("meeting_resource_scopes", []string{})
		context.Next()
	})
	NewMeetingV2Handler(store, worker, apiProvider{}, 100).Register(router.Group("/api/v1/meeting-sessions"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/bob-meeting", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("global scope crossed meeting boundary: status=%d", response.Code)
	}
}

func TestMeetingV2CapabilitiesAvailableOnlyWhenRouteRegistered(t *testing.T) {
	router, _, _, _ := setupMeetingV2Router(t, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/capabilities", nil))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"enabled":true`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMeetingV2TranscriptPaginationValidation(t *testing.T) {
	router, store, _, _ := setupMeetingV2Router(t, []string{users.ScopeMeetingRead})
	if _, err := store.CreateOwnedSession("transcript-session", "", "alice", nil); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/transcript-session/transcript?after=-2", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMeetingV2WithoutASRSupportsManualTranscriptAndHealth(t *testing.T) {
	scopes := []string{users.ScopeMeetingRead, users.ScopeMeetingWrite}
	router, _, _ := setupMeetingV2RouterWithoutASR(t, scopes)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/meeting-sessions", bytes.NewBufferString(`{"title":"manual"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var payload struct {
		Session meetingv2.Session `json:"session"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Session.TranscriptionMode != meetingv2.TranscriptionManual {
		t.Fatalf("mode=%q", payload.Session.TranscriptionMode)
	}

	health := httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/asr/health", nil)
	healthResponse := httptest.NewRecorder()
	router.ServeHTTP(healthResponse, health)
	if healthResponse.Code != http.StatusOK || !bytes.Contains(healthResponse.Body.Bytes(), []byte(`"configured":false`)) {
		t.Fatalf("health status=%d body=%s", healthResponse.Code, healthResponse.Body.String())
	}

	manual := httptest.NewRequest(http.MethodPut, "/api/v1/meeting-sessions/"+payload.Session.ID+"/transcript/manual", bytes.NewBufferString(`{"text":"人工记录","language":"zh","expected_revision":0}`))
	manual.Header.Set("Content-Type", "application/json")
	manual.Header.Set("Idempotency-Key", "manual-1")
	manualResponse := httptest.NewRecorder()
	router.ServeHTTP(manualResponse, manual)
	if manualResponse.Code != http.StatusCreated {
		t.Fatalf("manual status=%d body=%s", manualResponse.Code, manualResponse.Body.String())
	}
}

func TestMeetingV2WithoutASRImportsCompleteAudio(t *testing.T) {
	scopes := []string{users.ScopeMeetingRead, users.ScopeMeetingWrite}
	router, _, _ := setupMeetingV2RouterWithoutASR(t, scopes)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/meeting-sessions", bytes.NewBufferString(`{"title":"import"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	var payload struct {
		Session meetingv2.Session `json:"session"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	hash := sha256.Sum256(audio)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/meeting-sessions/"+payload.Session.ID+"/audio", bytes.NewReader(audio))
	request.Header.Set("Content-Type", "audio/mpeg")
	request.Header.Set("X-AIDG-Import-ID", "import-1")
	request.Header.Set("X-AIDG-Audio-SHA256", hex.EncodeToString(hash[:]))
	request.Header.Set("X-AIDG-Audio-Duration-Ms", "1000")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || !bytes.Contains(response.Body.Bytes(), []byte(`"state":"completed"`)) {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMeetingV2DocumentAPIProtectsUserRevision(t *testing.T) {
	scopes := []string{users.ScopeMeetingRead, users.ScopeMeetingWrite}
	router, store, _ := setupMeetingV2RouterWithoutASR(t, scopes)
	if _, err := store.CreateOwnedSessionWithMode("document-api", "Docs", "alice", nil, meetingv2.TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	put := func(content string, revision int, key string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"content":%q,"expected_revision":%d,"source":"external"}`, content, revision)
		request := httptest.NewRequest(http.MethodPut, "/api/v1/meeting-sessions/document-api/documents/meeting_summary", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", key)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	first := put("summary", 0, "summary-1")
	if first.Code != http.StatusCreated || !bytes.Contains(first.Body.Bytes(), []byte(`"active":true`)) {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	replay := put("summary", 0, "summary-1")
	if replay.Code != http.StatusOK || !bytes.Contains(replay.Body.Bytes(), []byte(`"duplicate":true`)) {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/meeting-sessions/document-api/documents/meeting_summary", nil)
	got := httptest.NewRecorder()
	router.ServeHTTP(got, get)
	if got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte(`"content":"summary"`)) {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestValidMeetingMetadataRejectsMigrationReservedKeys(t *testing.T) {
	for _, key := range []string{"migration_source", "migration_fingerprint", "legacy_source_id"} {
		if validMeetingMetadata(map[string]string{key: "legacy"}) {
			t.Fatalf("reserved migration metadata key accepted: %s", key)
		}
	}
	if !validMeetingMetadata(map[string]string{"task_id": "task-1", "product_line": "NISG"}) {
		t.Fatal("ordinary metadata rejected")
	}
}
