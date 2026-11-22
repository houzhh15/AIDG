package meetingv2

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testProvider(t *testing.T, handler http.HandlerFunc, profile string) *OpenAICompatibleProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	host, _, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewOpenAICompatibleProvider(ProviderOptions{
		Profile: profile, Endpoint: server.URL, Model: "whisper-test", APIKey: "secret",
		RequestTimeout: 5 * time.Second, HealthTimeout: time.Second,
		AllowedHosts: []string{host},
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestProviderTranscribeMultipart(t *testing.T) {
	provider := testProvider(t, func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing authorization")
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "audio" || request.FormValue("model") != "whisper-test" || request.FormValue("response_format") != "json" {
			t.Errorf("unexpected multipart request")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"text":"hello","language":"en","duration":1.25}`))
	}, "omlx")
	result, err := provider.Transcribe(context.Background(), TranscribeRequest{Audio: bytes.NewReader([]byte("audio")), Filename: "x.webm"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "hello" || result.DurationMS != 1250 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestProviderClassifiesStatusAndRejectsTrailingJSON(t *testing.T) {
	status := http.StatusServiceUnavailable
	provider := testProvider(t, func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(status)
		_, _ = response.Write([]byte("external secret details"))
	}, "ollama")
	_, err := provider.Transcribe(context.Background(), TranscribeRequest{Audio: bytes.NewReader([]byte("audio"))})
	if !errors.Is(err, ErrProviderRetryable) || strings.Contains(err.Error(), "secret details") {
		t.Fatalf("unexpected retryable error: %v", err)
	}
	status = http.StatusUnauthorized
	_, err = provider.Transcribe(context.Background(), TranscribeRequest{Audio: bytes.NewReader([]byte("audio"))})
	if !errors.Is(err, ErrProviderPermanent) {
		t.Fatalf("unexpected permanent error: %v", err)
	}

	provider = testProvider(t, func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"text":"a"}{"text":"b"}`))
	}, "omlx")
	_, err = provider.Transcribe(context.Background(), TranscribeRequest{Audio: bytes.NewReader([]byte("audio"))})
	if !errors.Is(err, ErrProviderPermanent) {
		t.Fatalf("expected trailing JSON rejection, got %v", err)
	}
}

func TestProviderHealthProfilesAndRedirect(t *testing.T) {
	var pathSeen string
	provider := testProvider(t, func(response http.ResponseWriter, request *http.Request) {
		pathSeen = request.URL.Path
		_, _ = response.Write([]byte(`{"models":[]}`))
	}, "ollama")
	if err := provider.Check(context.Background()); err != nil || pathSeen != "/api/tags" {
		t.Fatalf("ollama health path=%s err=%v", pathSeen, err)
	}

	provider = testProvider(t, func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/elsewhere", http.StatusFound)
	}, "omlx")
	if err := provider.Check(context.Background()); err == nil {
		t.Fatal("expected redirect to be rejected")
	}
}

func TestProviderRejectsUnsafeEndpoint(t *testing.T) {
	base := ProviderOptions{Profile: "ollama", Model: "whisper", RequestTimeout: time.Second, HealthTimeout: time.Second, AllowedHosts: []string{"localhost"}}
	for _, endpoint := range []string{"file:///tmp/asr", "http://user:password@localhost:1", "http://localhost:1/path", "http://localhost:1?q=1"} {
		base.Endpoint = endpoint
		if _, err := NewOpenAICompatibleProvider(base); err == nil {
			t.Errorf("expected endpoint rejection: %s", endpoint)
		}
	}
	base.Endpoint = "http://not-allowed.invalid:1"
	if _, err := NewOpenAICompatibleProvider(base); err == nil {
		t.Fatal("expected non-allowlisted hostname rejection")
	}
}
