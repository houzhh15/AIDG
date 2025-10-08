package api

// meetings_chunk.go - Meeting audio chunk processing operations
// Handles: MergeChunk, ChunkDebug, RedoSpeakers, RedoEmbeddings, RedoMapped
// Includes: applyLocalMapping, applyGlobalMapping, resolveChunkFile, srvSpeakersFile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
)

// ============================================================================
// Chunk Merge Handler
// ============================================================================

// HandleMergeChunk POST /api/v1/tasks/:id/chunks/:cid/merge
// 合并单个 chunk (segments + speakers → merged.txt)
func HandleMergeChunk(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Resolve needed segment & speaker files
		baseDir := t.Cfg.OutputDir
		n, err := strconv.Atoi(cid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk id"})
			return
		}

		// Construct file paths
		pad := fmt.Sprintf("%04d", n)
		seg := filepath.Join(baseDir, fmt.Sprintf("chunk_%s_segments.json", pad))
		spkBase := filepath.Join(baseDir, fmt.Sprintf("chunk_%s_speakers.json", pad))

		// Check merge-segments binary exists
		if _, err := os.Stat("go-whisper/merge-segments"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "merge-segments binary not found",
				"detail": "expected at go-whisper/merge-segments",
			})
			return
		}

		// Check segments file exists
		if _, err := os.Stat(seg); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "segments missing"})
			return
		}

		// Prefer mapped_global > mapped > base speakers file
		spk := spkBase
		g := strings.Replace(spkBase, "_speakers.json", "_speakers_mapped_global.json", 1)
		m := strings.Replace(spkBase, "_speakers.json", "_speakers_mapped.json", 1)
		if _, err := os.Stat(g); err == nil {
			spk = g
		} else if _, err := os.Stat(m); err == nil {
			spk = m
		}

		// Check speaker file exists
		if _, err := os.Stat(spk); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "speaker file missing"})
			return
		}

		// Sanity size check
		if infoSeg, err2 := os.Stat(seg); err2 == nil && infoSeg.Size() == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty segments file"})
			return
		}
		if infoSpk, err2 := os.Stat(spk); err2 == nil && infoSpk.Size() == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty speaker file"})
			return
		}

		// Execute merge command
		out := filepath.Join(baseDir, fmt.Sprintf("chunk_%s_merged.txt", pad))
		args := []string{"go-whisper/merge-segments", "--segments-file", seg, "--speaker-file", spk}
		cmd := exec.Command(args[0], args[1:]...)
		f, _ := os.Create(out)
		cmd.Stdout = f
		var stderrBuf strings.Builder
		cmd.Stderr = &stderrBuf
		err = cmd.Run()
		f.Close()

		if err != nil {
			// Delete potentially incomplete output file
			_ = os.Remove(out)

			// Collect extra diagnostics
			segSz, spkSz := int64(-1), int64(-1)
			if infoSeg, e2 := os.Stat(seg); e2 == nil {
				segSz = infoSeg.Size()
			}
			if infoSpk, e2 := os.Stat(spk); e2 == nil {
				spkSz = infoSpk.Size()
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "merge failed",
				"detail":   stderrBuf.String(),
				"cmd":      strings.Join(args, " "),
				"segments": filepath.Base(seg),
				"speakers": filepath.Base(spk),
				"seg_size": segSz,
				"spk_size": spkSz,
			})
			return
		}

		// Remove empty lines (including lines with only whitespace)
		if b, rerr := os.ReadFile(out); rerr == nil {
			lines := strings.Split(string(b), "\n")
			filtered := make([]string, 0, len(lines))
			for _, l := range lines {
				if strings.TrimSpace(l) == "" {
					continue
				}
				filtered = append(filtered, l)
			}
			if len(filtered) > 0 {
				_ = os.WriteFile(out, []byte(strings.Join(filtered, "\n")+"\n"), 0o644)
			}
		}

		c.JSON(http.StatusOK, gin.H{"merged": filepath.Base(out)})
	}
}

// ============================================================================
// Chunk Operations Handlers
// ============================================================================

// srvSpeakersFile is internal type for speaker mapping operations
type srvSpeakersFile struct {
	Segments []struct {
		Start, End float64
		Speaker    string `json:"speaker"`
	} `json:"segments"`
}

// applyLocalMapping applies local speaker mapping from embeddings file
func applyLocalMapping(speakersPath, embeddingsPath string) (string, error) {
	f, err := os.Open(embeddingsPath)
	if err != nil {
		return speakersPath, err
	}
	defer f.Close()

	var raw map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return speakersPath, err
	}

	lom, ok := raw["local_original_mapping"].(map[string]any)
	if !ok {
		return speakersPath, nil
	}

	spFile, err := os.Open(speakersPath)
	if err != nil {
		return "", err
	}
	defer spFile.Close()

	var spData srvSpeakersFile
	if err := json.NewDecoder(spFile).Decode(&spData); err != nil {
		return "", err
	}

	repl := map[string]string{}
	for k, v := range lom {
		if vs, ok := v.(string); ok {
			repl[k] = vs
		}
	}

	for i := range spData.Segments {
		if newSpk, ok := repl[spData.Segments[i].Speaker]; ok {
			spData.Segments[i].Speaker = newSpk
		}
	}

	outPath := strings.TrimSuffix(speakersPath, ".json") + "_mapped.json"
	of, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer of.Close()

	enc := json.NewEncoder(of)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spData); err != nil {
		return "", err
	}

	return outPath, nil
}

// applyGlobalMapping applies global speaker mapping from embeddings file
func applyGlobalMapping(speakersPath, embeddingsPath string) (string, error) {
	f, err := os.Open(embeddingsPath)
	if err != nil {
		return speakersPath, err
	}
	defer f.Close()

	var raw map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return speakersPath, err
	}

	mp, ok := raw["mapping"].(map[string]any)
	if !ok || len(mp) == 0 {
		return speakersPath, nil
	}

	spFile, err := os.Open(speakersPath)
	if err != nil {
		return speakersPath, err
	}
	defer spFile.Close()

	var spData srvSpeakersFile
	if err := json.NewDecoder(spFile).Decode(&spData); err != nil {
		return speakersPath, err
	}

	repl := map[string]string{}
	for k, v := range mp {
		if vs, ok := v.(string); ok {
			repl[k] = vs
		}
	}

	changed := false
	for i := range spData.Segments {
		if newSpk, ok := repl[spData.Segments[i].Speaker]; ok && newSpk != spData.Segments[i].Speaker {
			spData.Segments[i].Speaker = newSpk
			changed = true
		}
	}

	if !changed {
		return speakersPath, nil
	}

	outPath := strings.TrimSuffix(speakersPath, ".json") + "_global.json"
	of, err := os.Create(outPath)
	if err != nil {
		return speakersPath, err
	}
	defer of.Close()

	enc := json.NewEncoder(of)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spData); err != nil {
		return speakersPath, err
	}

	return outPath, nil
}

// HandleChunkDebug GET /api/v1/tasks/:id/chunks/:cid/debug
// 获取 chunk 相关文件的调试信息 (存在性和大小)
func HandleChunkDebug(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		n, err := strconv.Atoi(cid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk id"})
			return
		}

		pad := fmt.Sprintf("%04d", n)
		base := t.Cfg.OutputDir

		// Check all chunk-related files
		paths := map[string]string{
			"wav":        filepath.Join(base, fmt.Sprintf("chunk_%s.wav", pad)),
			"segments":   filepath.Join(base, fmt.Sprintf("chunk_%s_segments.json", pad)),
			"speakers":   filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad)),
			"mapped":     filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped.json", pad)),
			"global":     filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped_global.json", pad)),
			"embeddings": filepath.Join(base, fmt.Sprintf("chunk_%s_embeddings.json", pad)),
			"merged":     filepath.Join(base, fmt.Sprintf("chunk_%s_merged.txt", pad)),
		}

		info := gin.H{}
		for k, p := range paths {
			if st, err := os.Stat(p); err == nil {
				info[k] = gin.H{"exists": true, "size": st.Size(), "name": filepath.Base(p)}
			} else {
				info[k] = gin.H{"exists": false}
			}
		}

		c.JSON(http.StatusOK, gin.H{"chunk": cid, "files": info})
	}
}

// HandleRedoSpeakers POST /api/v1/tasks/:id/chunks/:cid/redo/speakers
// 重新识别说话人 (使用 SpeechBrain 或 Pyannote)
func HandleRedoSpeakers(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		base := t.Cfg.OutputDir
		n, err := strconv.Atoi(cid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk id"})
			return
		}

		pad := fmt.Sprintf("%04d", n)
		wav := filepath.Join(base, fmt.Sprintf("chunk_%s.wav", pad))
		if _, err := os.Stat(wav); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "wav missing"})
			return
		}

		// Remove old related mapping/global files to avoid confusion
		speakersPath := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad))
		_ = os.Remove(speakersPath)
		_ = os.Remove(strings.Replace(speakersPath, "_speakers.json", "_speakers_mapped.json", 1))
		_ = os.Remove(strings.Replace(speakersPath, "_speakers.json", "_speakers_mapped_global.json", 1))

		// Build command based on diarization backend
		var args []string
		if t.Cfg.DiarizationBackend == "speechbrain" {
			args = []string{"python3", "speechbrain/speechbrain_diarize.py", "--input", wav, "--device", t.Cfg.DeviceDefault}

			// SpeechBrain specific parameters
			if t.Cfg.SBEnergyVAD {
				args = append(args, "--energy_vad", "--energy_vad_thr", fmt.Sprintf("%g", t.Cfg.SBEnergyVADThr))
			}
			if t.Cfg.SBOverclusterFactor > 1.0 {
				args = append(args, "--overcluster_factor", fmt.Sprintf("%g", t.Cfg.SBOverclusterFactor))
			}
			if t.Cfg.SBMergeThreshold > 0 {
				args = append(args, "--merge_threshold", fmt.Sprintf("%g", t.Cfg.SBMergeThreshold))
			}
			if t.Cfg.SBMinSegmentMerge > 0 {
				args = append(args, "--min_segment_merge", fmt.Sprintf("%g", t.Cfg.SBMinSegmentMerge))
			}
			if t.Cfg.SBReassignAfterMerge {
				args = append(args, "--reassign_after_merge")
			}
		} else {
			// Pyannote backend
			args = []string{"python3", "pyannote/pyannote_diarize.py", "--input", wav, "--device", t.Cfg.DeviceDefault}
			if t.Cfg.EnableOffline {
				args = append(args, "--offline")
			}
		}

		// Execute diarization command
		cmd := exec.Command(args[0], args[1:]...)
		env := os.Environ()
		if t.Cfg.DiarizationBackend == "pyannote" {
			env = append(env, "HUGGINGFACE_TOKEN="+t.Cfg.HFTokenValue)
			if t.Cfg.EnableOffline {
				env = append(env, "HF_HUB_OFFLINE=1")
			}
		}
		cmd.Env = env

		outF, _ := os.Create(speakersPath)
		cmd.Stdout = outF
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		outF.Close()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "redo speakers failed", "detail": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"speakers": filepath.Base(speakersPath)})
	}
}

// HandleRedoEmbeddings POST /api/v1/tasks/:id/chunks/:cid/redo/embeddings
// 重新生成 embeddings (requires existing speakers.json)
func HandleRedoEmbeddings(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		base := t.Cfg.OutputDir
		n, err := strconv.Atoi(cid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk id"})
			return
		}

		pad := fmt.Sprintf("%04d", n)
		wav := filepath.Join(base, fmt.Sprintf("chunk_%s.wav", pad))
		if _, err := os.Stat(wav); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "wav missing"})
			return
		}

		speakers := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad))
		if _, err := os.Stat(speakers); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "speakers missing"})
			return
		}

		embPath := filepath.Join(base, fmt.Sprintf("chunk_%s_embeddings.json", pad))
		_ = os.Remove(embPath)

		// Build embedding generation command
		args := []string{
			"python3", t.Cfg.EmbeddingScriptPath,
			"--audio", wav,
			"--speakers-json", speakers,
			"--output", embPath,
			"--device", t.Cfg.EmbeddingDeviceDefault,
			"--threshold", t.Cfg.EmbeddingThreshold,
			"--auto-lower-threshold",
			"--auto-lower-min", t.Cfg.EmbeddingAutoLowerMin,
			"--auto-lower-step", t.Cfg.EmbeddingAutoLowerStep,
			"--hf_token", t.Cfg.HFTokenValue,
		}

		if t.Cfg.EnableOffline && strings.Contains(t.Cfg.EmbeddingScriptPath, "pyannote/") {
			args = append(args, "--offline")
		}

		// Find latest embeddings file from other chunks (for existing-embeddings)
		entries, _ := os.ReadDir(base)
		latest := ""
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, "_embeddings.json") && !strings.Contains(name, pad+"_embeddings") {
				latest = filepath.Join(base, name)
			}
		}
		if latest != "" {
			args = append(args, "--existing-embeddings", latest)
		}

		// Execute embedding generation command
		cmd := exec.Command(args[0], args[1:]...)
		env := os.Environ()
		if t.Cfg.DiarizationBackend == "pyannote" {
			env = append(env, "HUGGINGFACE_TOKEN="+t.Cfg.HFTokenValue)
			if t.Cfg.EnableOffline {
				env = append(env, "HF_HUB_OFFLINE=1")
			}
		}
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "redo embeddings failed", "detail": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"embeddings": filepath.Base(embPath)})
	}
}

// HandleRedoMapped POST /api/v1/tasks/:id/chunks/:cid/redo/mapped
// 重新映射说话人 (local + global mapping using existing embeddings & speakers)
func HandleRedoMapped(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		base := t.Cfg.OutputDir
		n, err := strconv.Atoi(cid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk id"})
			return
		}

		pad := fmt.Sprintf("%04d", n)
		speakers := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad))
		emb := filepath.Join(base, fmt.Sprintf("chunk_%s_embeddings.json", pad))

		if _, err := os.Stat(speakers); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "speakers missing"})
			return
		}
		if _, err := os.Stat(emb); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "embeddings missing"})
			return
		}

		// Remove old mapped files
		mapped := strings.Replace(speakers, "_speakers.json", "_speakers_mapped.json", 1)
		global := strings.Replace(speakers, "_speakers.json", "_speakers_mapped_global.json", 1)
		_ = os.Remove(mapped)
		_ = os.Remove(global)

		// Apply local mapping
		if p, err := applyLocalMapping(speakers, emb); err == nil && p != "" {
			mapped = p
		}

		// Apply global mapping
		if p, err := applyGlobalMapping(mapped, emb); err == nil && p != "" {
			global = p
		}

		c.JSON(http.StatusOK, gin.H{"mapped": filepath.Base(mapped), "global": filepath.Base(global)})
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// resolveChunkFile maps kind to filename with priority logic
func resolveChunkFile(baseDir string, cid string, kind string) (string, string, error) {
	n, err := strconv.Atoi(cid)
	if err != nil {
		return "", "", err
	}
	fname := ""
	pad := fmt.Sprintf("%04d", n)
	switch kind {
	case "segments":
		fname = fmt.Sprintf("chunk_%s_segments.json", pad)
	case "speakers":
		fname = fmt.Sprintf("chunk_%s_speakers.json", pad)
	case "embeddings":
		fname = fmt.Sprintf("chunk_%s_embeddings.json", pad)
	case "mapped":
		g := fmt.Sprintf("chunk_%s_speakers_mapped_global.json", pad)
		m := fmt.Sprintf("chunk_%s_speakers_mapped.json", pad)
		if _, err := os.Stat(filepath.Join(baseDir, g)); err == nil {
			fname = g
		} else if _, err := os.Stat(filepath.Join(baseDir, m)); err == nil {
			fname = m
		} else {
			fname = m
		}
	case "merged":
		fname = fmt.Sprintf("chunk_%s_merged.txt", pad)
	default:
		return "", "", fmt.Errorf("invalid kind")
	}
	full := filepath.Join(baseDir, fname)
	return full, fname, nil
}
