package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckDiarizationEnv runs environment checks for the specified diarization backend.
// Returns detailed error with remediation guidance if any required dependency is missing.
func CheckDiarizationEnv(backend string) error {
	switch backend {
	case "speechbrain":
		return CheckSpeechBrainEnv()
	case "pyannote":
		return CheckPyAnnoteEnv()
	default:
		return fmt.Errorf("unsupported diarization backend: %s", backend)
	}
}

// CheckPyAnnoteEnv checks PyAnnote environment and dependencies
func CheckPyAnnoteEnv() error {
	type dep struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
		Ok       bool   `json:"ok"`
		Version  string `json:"version"`
		Error    string `json:"error"`
	}
	type report struct {
		Status string                 `json:"status"`
		Deps   []dep                  `json:"deps"`
		Model  map[string]interface{} `json:"model"`
	}

	scripts := []string{
		"pyannote/pyannote_diarize.py",
		"pyannote/generate_speaker_embeddings.py",
	}
	var problems []string
	var advices []string

	for _, script := range scripts {
		if _, err := os.Stat(script); err != nil {
			problems = append(problems, fmt.Sprintf("script missing: %s", script))
			advices = append(advices, "确认已在项目根目录运行，且包含 pyannote 目录")
			continue
		}

		cmd := exec.Command("python3", script, "--check")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			problems = append(problems, fmt.Sprintf("check run failed for %s: %v (stderr=%s)", filepath.Base(script), err, trimNL(stderr.String())))
			advices = append(advices, fmt.Sprintf("尝试执行: python3 %s --check 单独查看错误", script))
			continue
		}

		var rep report
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			problems = append(problems, fmt.Sprintf("invalid JSON from %s: %v", filepath.Base(script), err))
			advices = append(advices, fmt.Sprintf("查看输出: %s", truncate(stdout.String(), 400)))
			continue
		}

		// Check required dependencies
		for _, d := range rep.Deps {
			if d.Required && !d.Ok {
				problems = append(problems, fmt.Sprintf("%s missing (script=%s): %s", d.Name, filepath.Base(script), firstNonEmpty(d.Error, "未安装")))
			}
		}

		// Check model cache
		if rep.Model != nil {
			if cached, ok := rep.Model["cached"].(bool); ok && !cached {
				problems = append(problems, fmt.Sprintf("model not cached (script=%s)", filepath.Base(script)))
				advices = append(advices, fmt.Sprintf("执行一次联网缓存: python3 %s --download", script))
			}
			if me, ok := rep.Model["error"].(string); ok && me != "" {
				problems = append(problems, fmt.Sprintf("model error (script=%s): %s", filepath.Base(script), me))
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}

	// Consolidate advices (dedupe)
	adviceSet := map[string]struct{}{}
	for _, a := range advices {
		if a == "" {
			continue
		}
		adviceSet[a] = struct{}{}
	}
	finalAdv := make([]string, 0, len(adviceSet))
	for a := range adviceSet {
		finalAdv = append(finalAdv, a)
	}

	errMsg := strings.Builder{}
	errMsg.WriteString("PyAnnote 环境检查失败:\n")
	for i, p := range problems {
		errMsg.WriteString(fmt.Sprintf("  %d) %s\n", i+1, p))
	}
	if len(finalAdv) > 0 {
		errMsg.WriteString("建议:\n")
		for _, a := range finalAdv {
			errMsg.WriteString("  - " + a + "\n")
		}
		errMsg.WriteString("  - 若无 GPU 可忽略 cuda/mps 不可用提示。\n")
		errMsg.WriteString("  - 安装依赖示例: pip install pyannote.audio torch torchaudio librosa soundfile numpy\n")
	}
	return errors.New(errMsg.String())
}
