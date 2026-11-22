package meetingv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const maxProviderResponseSize = 1 << 20
const maxProviderErrorSize = 4 << 10

var (
	ErrProviderRetryable = errors.New("ASR provider retryable error")
	ErrProviderPermanent = errors.New("ASR provider permanent error")
)

type ASRProvider interface {
	Transcribe(ctx context.Context, request TranscribeRequest) (TranscribeResult, error)
	Check(ctx context.Context) error
}

type TranscribeRequest struct {
	Audio    io.Reader
	Filename string
	Model    string
	Language string
}

type TranscribeResult struct {
	Text       string
	Language   string
	DurationMS int64
}

type ProviderOptions struct {
	Profile        string
	Endpoint       string
	Model          string
	Language       string
	APIKey         string
	RequestTimeout time.Duration
	HealthTimeout  time.Duration
	AllowedHosts   []string
	AllowedCIDRs   []string
}

type OpenAICompatibleProvider struct {
	profile       string
	baseURL       *url.URL
	model         string
	language      string
	apiKey        string
	client        *http.Client
	healthTimeout time.Duration
}

func NewOpenAICompatibleProvider(options ProviderOptions) (*OpenAICompatibleProvider, error) {
	baseURL, err := url.Parse(options.Endpoint)
	if err != nil || baseURL.Scheme == "" || baseURL.Hostname() == "" {
		return nil, fmt.Errorf("invalid ASR endpoint")
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid ASR endpoint scheme")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" || (baseURL.Path != "" && baseURL.Path != "/") {
		return nil, fmt.Errorf("ASR endpoint must be an origin without credentials, query, fragment, or API path")
	}
	if options.Profile != "ollama" && options.Profile != "omlx" {
		return nil, fmt.Errorf("unsupported ASR profile %q", options.Profile)
	}
	if strings.TrimSpace(options.Model) == "" {
		return nil, fmt.Errorf("ASR model is required")
	}
	policy, err := newNetworkPolicy(options.AllowedHosts, options.AllowedCIDRs)
	if err != nil {
		return nil, err
	}
	if !policy.hostAllowed(baseURL.Hostname()) {
		return nil, fmt.Errorf("ASR endpoint host is not allowlisted")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: minDuration(options.RequestTimeout, 30*time.Second), KeepAlive: 30 * time.Second}
	transport.DialContext = policy.dialContext(dialer)
	client := &http.Client{
		Transport: transport,
		Timeout:   options.RequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &OpenAICompatibleProvider{
		profile:       options.Profile,
		baseURL:       baseURL,
		model:         options.Model,
		language:      options.Language,
		apiKey:        options.APIKey,
		client:        client,
		healthTimeout: options.HealthTimeout,
	}, nil
}

func (p *OpenAICompatibleProvider) Transcribe(ctx context.Context, request TranscribeRequest) (TranscribeResult, error) {
	if request.Audio == nil {
		return TranscribeResult{}, fmt.Errorf("%w: missing audio", ErrProviderPermanent)
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	filename := path.Base(request.Filename)
	if filename == "." || filename == "" || filename == "/" {
		filename = "audio.webm"
	}
	model := request.Model
	if model == "" {
		model = p.model
	}
	language := request.Language
	if language == "" {
		language = p.language
	}
	go func() {
		var writeErr error
		defer func() {
			if closeErr := writer.Close(); writeErr == nil {
				writeErr = closeErr
			}
			_ = pipeWriter.CloseWithError(writeErr)
		}()
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			writeErr = err
			return
		}
		if _, err := io.Copy(part, request.Audio); err != nil {
			writeErr = err
			return
		}
		if err := writer.WriteField("model", model); err != nil {
			writeErr = err
			return
		}
		if language != "" && language != "auto" {
			if err := writer.WriteField("language", language); err != nil {
				writeErr = err
				return
			}
		}
		writeErr = writer.WriteField("response_format", "json")
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.resolve("/v1/audio/transcriptions"), pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return TranscribeResult{}, fmt.Errorf("%w: build request", ErrProviderPermanent)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	p.authorize(req)
	response, err := p.client.Do(req)
	_ = pipeReader.Close()
	if err != nil {
		if ctx.Err() != nil {
			return TranscribeResult{}, ctx.Err()
		}
		return TranscribeResult{}, fmt.Errorf("%w: request failed", ErrProviderRetryable)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderErrorSize))
		if retryableStatus(response.StatusCode) {
			return TranscribeResult{}, fmt.Errorf("%w: HTTP %d", ErrProviderRetryable, response.StatusCode)
		}
		return TranscribeResult{}, fmt.Errorf("%w: HTTP %d", ErrProviderPermanent, response.StatusCode)
	}

	var payload struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
	}
	if err := decodeBoundedJSON(response.Body, maxProviderResponseSize, &payload); err != nil {
		return TranscribeResult{}, fmt.Errorf("%w: invalid response", ErrProviderPermanent)
	}
	if !utf8.ValidString(payload.Text) || len(payload.Text) > maxProviderResponseSize || len(payload.Language) > 32 || payload.Duration < 0 || payload.Duration > 86400 {
		return TranscribeResult{}, fmt.Errorf("%w: invalid transcription fields", ErrProviderPermanent)
	}
	return TranscribeResult{Text: payload.Text, Language: payload.Language, DurationMS: int64(payload.Duration * 1000)}, nil
}

func (p *OpenAICompatibleProvider) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.healthTimeout)
	defer cancel()
	healthPath := "/v1/models"
	if p.profile == "ollama" {
		healthPath = "/api/tags"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.resolve(healthPath), nil)
	if err != nil {
		return err
	}
	p.authorize(req)
	response, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("ASR health request failed")
	}
	defer response.Body.Close()
	written, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderResponseSize+1))
	if readErr != nil || written > maxProviderResponseSize {
		return fmt.Errorf("ASR health response failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("ASR health HTTP %d", response.StatusCode)
	}
	return nil
}

func (p *OpenAICompatibleProvider) resolve(apiPath string) string {
	resolved := *p.baseURL
	resolved.Path = apiPath
	return resolved.String()
}

func (p *OpenAICompatibleProvider) authorize(request *http.Request) {
	if p.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

type networkPolicy struct {
	hosts map[string]bool
	cidrs []*net.IPNet
}

func newNetworkPolicy(hosts, cidrs []string) (*networkPolicy, error) {
	policy := &networkPolicy{hosts: make(map[string]bool)}
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || strings.ContainsAny(host, "/:@") {
			return nil, fmt.Errorf("invalid allowed host")
		}
		policy.hosts[host] = true
	}
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed CIDR")
		}
		policy.cidrs = append(policy.cidrs, network)
	}
	if len(policy.hosts) == 0 && len(policy.cidrs) == 0 {
		return nil, fmt.Errorf("ASR network allowlist is empty")
	}
	return policy, nil
}

func (p *networkPolicy) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if p.hosts[host] {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && p.ipAllowed(ip)
}

func (p *networkPolicy) ipAllowed(ip net.IP) bool {
	for _, network := range p.cidrs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (p *networkPolicy) dialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	resolver := net.DefaultResolver
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid ASR target")
		}
		explicitHost := p.hosts[strings.ToLower(strings.TrimSuffix(host, "."))]
		ips, err := resolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("resolve ASR target")
		}
		// Hostnames permit service discovery, while CIDRs constrain where those
		// names may resolve. When CIDRs are configured, both checks are required.
		if !explicitHost && len(p.cidrs) == 0 {
			return nil, fmt.Errorf("ASR target host is not allowlisted")
		}
		if len(p.cidrs) > 0 {
			for _, resolved := range ips {
				if !p.ipAllowed(resolved.IP) {
					return nil, fmt.Errorf("ASR target is outside allowed networks")
				}
			}
		}
		// Dial the already-validated address directly so the system resolver cannot
		// resolve the hostname a second time between validation and connection.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status == 500 || status == 502 || status == 503 || status == 504
}

func decodeBoundedJSON(reader io.Reader, limit int64, target any) error {
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("response too large")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("response contains trailing data")
	}
	return nil
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
