package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/houzhh15/AIDG/cmd/server/internal/meetingv2"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

type MeetingV2Handler struct {
	store      *meetingv2.Store
	worker     *meetingv2.Worker
	provider   meetingv2.ASRProvider
	maxChunks  int
	asrEnabled bool
}

func NewMeetingV2Handler(store *meetingv2.Store, worker *meetingv2.Worker, provider meetingv2.ASRProvider, maxChunks int, asrEnabled ...bool) *MeetingV2Handler {
	enabled := provider != nil
	if len(asrEnabled) > 0 {
		enabled = asrEnabled[0]
	}
	return &MeetingV2Handler{store: store, worker: worker, provider: provider, maxChunks: maxChunks, asrEnabled: enabled}
}

func (h *MeetingV2Handler) Register(routes *gin.RouterGroup) {
	routes.GET("/capabilities", h.capabilities)
	routes.POST("", requireMeetingV2Scope(users.ScopeMeetingWrite), h.create)
	routes.GET("/:meeting_id", requireMeetingV2Scope(users.ScopeMeetingRead), h.get)
	routes.GET("/:meeting_id/transcript", requireMeetingV2Scope(users.ScopeMeetingRead), h.transcript)
	routes.GET("/:meeting_id/transcript/manual", requireMeetingV2Scope(users.ScopeMeetingRead), h.manualTranscript)
	routes.PUT("/:meeting_id/transcript/manual", requireMeetingV2Scope(users.ScopeMeetingWrite), h.submitManualTranscript)
	routes.GET("/:meeting_id/documents/:kind", requireMeetingV2Scope(users.ScopeMeetingRead), h.document)
	routes.PUT("/:meeting_id/documents/:kind", requireMeetingV2Scope(users.ScopeMeetingWrite), h.putDocument)
	routes.PUT("/:meeting_id/audio", requireMeetingV2Scope(users.ScopeMeetingWrite), h.importAudio)
	routes.GET("/:meeting_id/recovery", requireMeetingV2Scope(users.ScopeMeetingRead), h.recovery)
	routes.POST("/:meeting_id/actions/start", requireMeetingV2Scope(users.ScopeMeetingWrite), h.action(meetingv2.StateRecording))
	routes.POST("/:meeting_id/actions/stop", requireMeetingV2Scope(users.ScopeMeetingWrite), h.action(meetingv2.StateStopping))
	routes.POST("/:meeting_id/actions/cancel", requireMeetingV2Scope(users.ScopeMeetingWrite), h.action(meetingv2.StateCancelled))
	routes.PUT("/:meeting_id/chunks/:chunk_index", requireMeetingV2Scope(users.ScopeMeetingWrite), h.uploadChunk)
	routes.GET("/asr/health", requireMeetingV2Scope(users.ScopeMeetingRead), h.health)
}

func requireMeetingV2Scope(scope string) gin.HandlerFunc {
	return func(context *gin.Context) {
		value, exists := context.Get("scopes")
		scopes, ok := value.([]string)
		if !exists || !ok || !users.HasScope(scopes, scope) {
			context.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		// Global meeting scopes are sufficient only for creation. Resource routes
		// must also receive owner/ACL-derived scopes from the auth middleware.
		if context.Param("meeting_id") != "" {
			injected, _ := context.Get("meeting_resource_scopes")
			resourceScopes, ok := injected.([]string)
			if !ok || !users.HasScope(resourceScopes, scope) {
				context.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		context.Next()
	}
}

func (h *MeetingV2Handler) create(context *gin.Context) {
	var request struct {
		Title    string            `json:"title"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeStrictJSON(http.MaxBytesReader(context.Writer, context.Request.Body, 64<<10), &request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if !utf8.ValidString(request.Title) || len([]rune(request.Title)) > 256 || !validMeetingMetadata(request.Metadata) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid meeting metadata"})
		return
	}
	owner, _ := context.Get("user")
	username, _ := owner.(string)
	if username == "" {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	mode := meetingv2.TranscriptionManual
	if h.asrEnabled {
		mode = meetingv2.TranscriptionAutomatic
	}
	session, err := h.store.CreateOwnedSessionWithMode(uuid.NewString(), request.Title, username, request.Metadata, mode)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "create meeting session failed"})
		return
	}
	context.JSON(http.StatusCreated, gin.H{"session": session})
}

func (h *MeetingV2Handler) get(context *gin.Context) {
	session, err := h.store.LoadSession(context.Param("meeting_id"))
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	status, err := h.store.GetRecoveryStatus(session.ID, 1000)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "read recovery status failed"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"session": session, "recovery": status})
}

func (h *MeetingV2Handler) recovery(context *gin.Context) {
	limit := 1000
	if raw := context.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}
	status, err := h.store.GetRecoveryStatus(context.Param("meeting_id"), limit)
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	context.JSON(http.StatusOK, status)
}

func (h *MeetingV2Handler) capabilities(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{
		"enabled":           true,
		"recording":         gin.H{"mime_types": []string{"audio/webm", "audio/ogg"}, "recommended_chunk_ms": 5000},
		"asr":               gin.H{"configured": h.asrEnabled},
		"manual_transcript": gin.H{"enabled": true, "max_bytes": 1 << 20},
		"audio_import":      gin.H{"enabled": true, "max_bytes": h.store.MaxChunkSize(), "mime_types": []string{"audio/webm", "audio/ogg", "audio/wav", "audio/mpeg", "audio/flac", "audio/mp4"}},
	})
}

func (h *MeetingV2Handler) transcript(context *gin.Context) {
	after, limit := -1, 200
	var err error
	if raw := context.Query("after"); raw != "" {
		after, err = strconv.Atoi(raw)
		if err != nil || after < -1 {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid after cursor"})
			return
		}
	}
	if raw := context.Query("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 500 {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
	}
	page, err := h.store.GetTranscriptPage(context.Param("meeting_id"), after, limit)
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	context.Header("Cache-Control", "no-store")
	context.JSON(http.StatusOK, page)
}
func (h *MeetingV2Handler) action(state meetingv2.SessionState) gin.HandlerFunc {
	return func(context *gin.Context) {
		session, err := h.store.SetSessionState(context.Param("meeting_id"), state)
		if err != nil {
			writeMeetingV2Error(context, err)
			return
		}
		if h.worker != nil && state == meetingv2.StateStopping {
			h.worker.Wake()
		}
		if h.worker != nil && state == meetingv2.StateCancelled {
			h.worker.CancelSession(session.ID)
		}
		context.JSON(http.StatusOK, gin.H{"session": session})
	}
}

func (h *MeetingV2Handler) uploadChunk(context *gin.Context) {
	index, err := strconv.Atoi(context.Param("chunk_index"))
	if err != nil || index < 0 || index >= h.maxChunks {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
		return
	}
	format, ok := audioFormat(context.GetHeader("Content-Type"))
	if !ok {
		context.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "unsupported audio content type"})
		return
	}
	duration, err := strconv.ParseInt(context.GetHeader("X-AIDG-Chunk-Duration-Ms"), 10, 64)
	if err != nil || duration < 1 || duration > 300000 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk duration"})
		return
	}
	startedAt := time.Time{}
	if raw := context.GetHeader("X-AIDG-Chunk-Started-At"); raw != "" {
		startedAt, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk start time"})
			return
		}
	}
	final, err := strconv.ParseBool(defaultString(context.GetHeader("X-AIDG-Chunk-Final"), "false"))
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid final flag"})
		return
	}
	result, err := h.store.IngestChunk(context.Param("meeting_id"), meetingv2.IngestChunkRequest{
		Index: index, ClientChunkID: context.GetHeader("X-AIDG-Chunk-ID"), StartedAt: startedAt,
		DurationMS: duration, Format: format, DeclaredSHA256: context.GetHeader("X-AIDG-Chunk-SHA256"), Final: final,
	}, http.MaxBytesReader(context.Writer, context.Request.Body, h.store.MaxChunkSize()))
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	if h.worker != nil {
		h.worker.Wake()
	}
	status := http.StatusAccepted
	if result.Duplicate {
		status = http.StatusOK
	}
	response := gin.H{"chunk": result.Chunk, "duplicate": result.Duplicate, "job_created": result.JobCreated}
	if result.JobState != nil {
		response["job_state"] = *result.JobState
	}
	context.JSON(status, response)
}

func (h *MeetingV2Handler) health(context *gin.Context) {
	if h.provider == nil {
		context.JSON(http.StatusOK, gin.H{"configured": false, "healthy": false, "checked_at": time.Now().UTC()})
		return
	}
	started := time.Now()
	err := h.provider.Check(context.Request.Context())
	payload := gin.H{"configured": true, "healthy": err == nil, "checked_at": time.Now().UTC(), "latency_ms": time.Since(started).Milliseconds()}
	if err != nil {
		context.JSON(http.StatusServiceUnavailable, payload)
		return
	}
	context.JSON(http.StatusOK, payload)
}

func (h *MeetingV2Handler) putDocument(context *gin.Context) {
	kind := meetingv2.DocumentKind(context.Param("kind"))
	var request struct {
		Content          string `json:"content"`
		ExpectedRevision int    `json:"expected_revision"`
		Source           string `json:"source"`
	}
	if err := decodeStrictJSON(http.MaxBytesReader(context.Writer, context.Request.Body, (2<<20)+(64<<10)), &request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	source := meetingv2.DocumentSourceUser
	if request.Source == string(meetingv2.DocumentSourceExternal) {
		source = meetingv2.DocumentSourceExternal
	} else if request.Source != "" && request.Source != string(meetingv2.DocumentSourceUser) {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid document source"})
		return
	}
	owner, _ := context.Get("user")
	username, _ := owner.(string)
	item, duplicate, err := h.store.PutDocument(context.Param("meeting_id"), meetingv2.PutDocumentRequest{
		Kind: kind, Source: source, Content: request.Content, ExpectedRevision: request.ExpectedRevision,
		IdempotencyKey: context.GetHeader("Idempotency-Key"), CreatedBy: username,
	})
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	context.JSON(status, gin.H{"document": item, "duplicate": duplicate})
}

func (h *MeetingV2Handler) document(context *gin.Context) {
	item, err := h.store.GetActiveDocument(context.Param("meeting_id"), meetingv2.DocumentKind(context.Param("kind")))
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"document": item})
}

func (h *MeetingV2Handler) submitManualTranscript(context *gin.Context) {
	var request struct {
		Text             string `json:"text"`
		Language         string `json:"language"`
		ExpectedRevision int    `json:"expected_revision"`
	}
	if err := decodeStrictJSON(http.MaxBytesReader(context.Writer, context.Request.Body, (1<<20)+(64<<10)), &request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	owner, _ := context.Get("user")
	username, _ := owner.(string)
	transcript, duplicate, err := h.store.SubmitManualTranscript(context.Param("meeting_id"), meetingv2.SubmitManualTranscriptRequest{
		Text: request.Text, Language: request.Language, ExpectedRevision: request.ExpectedRevision,
		IdempotencyKey: context.GetHeader("Idempotency-Key"), SubmittedBy: username,
	})
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	context.JSON(status, gin.H{"transcript": transcript, "duplicate": duplicate})
}

func (h *MeetingV2Handler) manualTranscript(context *gin.Context) {
	transcript, err := h.store.GetLatestManualTranscript(context.Param("meeting_id"))
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	context.JSON(http.StatusOK, gin.H{"transcript": transcript})
}

func (h *MeetingV2Handler) importAudio(context *gin.Context) {
	format, ok := meetingV2AudioFormat(context.GetHeader("Content-Type"))
	if !ok {
		context.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "unsupported audio format"})
		return
	}
	duration, err := strconv.ParseInt(context.GetHeader("X-AIDG-Audio-Duration-Ms"), 10, 64)
	importID := context.GetHeader("X-AIDG-Import-ID")
	declaredHash := context.GetHeader("X-AIDG-Audio-SHA256")
	if importID == "" || declaredHash == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "import ID and audio SHA-256 are required"})
		return
	}
	if err != nil || duration <= 0 || duration > 604800000 {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid audio duration"})
		return
	}
	var startedAt time.Time
	if value := context.GetHeader("X-AIDG-Audio-Started-At"); value != "" {
		startedAt, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid audio start time"})
			return
		}
	}
	result, session, err := h.store.ImportAudio(context.Param("meeting_id"), meetingv2.IngestChunkRequest{
		Index: 0, ClientChunkID: importID, StartedAt: startedAt,
		DurationMS: duration, Format: format, DeclaredSHA256: declaredHash, Final: true,
	}, http.MaxBytesReader(context.Writer, context.Request.Body, h.store.MaxChunkSize()))
	if err != nil {
		writeMeetingV2Error(context, err)
		return
	}
	if h.worker != nil {
		h.worker.Wake()
	}
	status := http.StatusAccepted
	if result.Duplicate {
		status = http.StatusOK
	}
	response := gin.H{"session": session, "chunk": result.Chunk, "duplicate": result.Duplicate, "job_created": result.JobCreated, "asr_job_created": result.JobCreated}
	if result.JobState != nil {
		response["job_state"] = *result.JobState
	}
	context.JSON(status, response)
}

func meetingV2AudioFormat(contentType string) (string, bool) {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	formats := map[string]string{"audio/webm": "webm", "audio/ogg": "ogg", "audio/wav": "wav", "audio/x-wav": "wav", "audio/mpeg": "mp3", "audio/flac": "flac", "audio/mp4": "m4a"}
	format, ok := formats[contentType]
	return format, ok
}

func decodeStrictJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func validMeetingMetadata(metadata map[string]string) bool {
	if len(metadata) > 32 {
		return false
	}
	for key, value := range metadata {
		if key == "migration_source" || key == "migration_fingerprint" || key == "legacy_source_id" {
			return false
		}
		if key == "" || len(key) > 64 || len(value) > 1024 || !utf8.ValidString(key) || !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

func audioFormat(contentType string) (string, bool) {
	return meetingV2AudioFormat(contentType)
}

func writeMeetingV2Error(context *gin.Context, err error) {
	var maxBytesError *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesError):
		context.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
	case errors.Is(err, meetingv2.ErrSessionNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "meeting session not found"})
	case errors.Is(err, meetingv2.ErrInvalidSessionID), errors.Is(err, meetingv2.ErrInvalidChunk), errors.Is(err, meetingv2.ErrInvalidTranscript), errors.Is(err, meetingv2.ErrInvalidDocument):
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid meeting request"})
	case errors.Is(err, meetingv2.ErrTranscriptNotFound), errors.Is(err, meetingv2.ErrDocumentNotFound):
		context.JSON(http.StatusNotFound, gin.H{"error": "meeting content not found"})
	case errors.Is(err, meetingv2.ErrChunkConflict):
		context.JSON(http.StatusConflict, gin.H{"error": "chunk conflict"})
	case errors.Is(err, meetingv2.ErrTranscriptConflict), errors.Is(err, meetingv2.ErrTranscriptRevisionConflict), errors.Is(err, meetingv2.ErrDocumentConflict), errors.Is(err, meetingv2.ErrDocumentRevisionConflict), errors.Is(err, meetingv2.ErrAmbiguousTaskSession):
		context.JSON(http.StatusConflict, gin.H{"error": "meeting content conflict"})
	default:
		context.JSON(http.StatusConflict, gin.H{"error": "meeting state conflict"})
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
