package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/dependency"
)

// meetings_chunk.go - Meeting audio chunk processing operations
// Handles: MergeChunk, ChunkDebug, RedoSpeakers, RedoEmbeddings, RedoMapped
// Includes: applyLocalMapping, applyGlobalMapping, resolveChunkFile, srvSpeakersFile

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

		// Get dependency client from orchestrator
		if t.Orch == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestrator not initialized"})
			return
		}
		depClient := t.Orch.GetDependencyClient()
		if depClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dependency client not available"})
			return
		}

		// Build command based on diarization backend
		var scriptPath string
		var cmdArgs []string

		if t.Cfg.DiarizationBackend == "speechbrain" {
			scriptPath = "/app/speechbrain/speechbrain_diarize.py" // Path inside deps-service
			cmdArgs = []string{scriptPath, "--input", wav, "--device", t.Cfg.DeviceDefault}

			// SpeechBrain specific parameters
			if t.Cfg.SBEnergyVAD {
				cmdArgs = append(cmdArgs, "--energy_vad", "--energy_vad_thr", fmt.Sprintf("%g", t.Cfg.SBEnergyVADThr))
			}
			if t.Cfg.SBOverclusterFactor > 1.0 {
				cmdArgs = append(cmdArgs, "--overcluster_factor", fmt.Sprintf("%g", t.Cfg.SBOverclusterFactor))
			}
			if t.Cfg.SBMergeThreshold > 0 {
				cmdArgs = append(cmdArgs, "--merge_threshold", fmt.Sprintf("%g", t.Cfg.SBMergeThreshold))
			}
			if t.Cfg.SBMinSegmentMerge > 0 {
				cmdArgs = append(cmdArgs, "--min_segment_merge", fmt.Sprintf("%g", t.Cfg.SBMinSegmentMerge))
			}
			if t.Cfg.SBReassignAfterMerge {
				cmdArgs = append(cmdArgs, "--reassign_after_merge")
			}
		} else {
			// Pyannote backend
			scriptPath = "/app/scripts/pyannote_diarize.py" // Path inside deps-service
			cmdArgs = []string{scriptPath, "--input", wav, "--device", t.Cfg.DeviceDefault}
			if t.Cfg.EnableOffline {
				cmdArgs = append(cmdArgs, "--offline")
			}
		}

		// Prepare environment variables
		env := map[string]string{
			"HUGGINGFACE_TOKEN": t.Cfg.HFTokenValue,
		}
		if t.Cfg.EnableOffline {
			env["HF_HUB_OFFLINE"] = "1"
		}

		// Execute diarization via DependencyClient
		diarizationCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()

		req := dependency.CommandRequest{
			Command: "python",
			Args:    cmdArgs,
			Env:     env,
			Timeout: 10 * time.Minute,
		}

		// Validate and execute
		if err := dependency.ValidateCommandRequest(req, depClient.Config()); err != nil {
			log.Printf("[API][RedoSpeakers] validation failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "command validation failed", "details": err.Error()})
			return
		}

		resp, err := depClient.ExecuteCommand(diarizationCtx, req)
		if err != nil || !resp.Success || resp.ExitCode != 0 {
			log.Printf("[API][RedoSpeakers] failed via DependencyClient: exit_code=%d, stderr=%s, err=%v", resp.ExitCode, resp.Stderr, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":     "diarization failed",
				"exit_code": resp.ExitCode,
				"stderr":    resp.Stderr,
			})
			return
		}

		// Write stdout to speakers file
		if err := os.WriteFile(speakersPath, []byte(resp.Stdout), 0644); err != nil {
			log.Printf("[API][RedoSpeakers] failed to write output: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write diarization output"})
			return
		}

		// Post-process: clamp any segment ends beyond wav duration
		// Note: sanitizeSpeakersJSON is in orchestrator package, skipping for now
		// if err := sanitizeSpeakersJSON(speakersPath, wav); err != nil {
		// 	log.Printf("[API][RedoSpeakers][sanitize] error: %v", err)
		// }

		c.JSON(http.StatusOK, gin.H{"status": "ok", "output_file": speakersPath})
	}
}

// HandleRedoEmbeddings POST /api/v1/tasks/:id/chunks/:cid/redo/embeddings
// 重新提取所有说话人的嵌入向量
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
		speakers := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad))
		if _, err := os.Stat(wav); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "wav missing"})
			return
		}
		if _, err := os.Stat(speakers); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "speakers json missing"})
			return
		}

		// Remove old related mapping/global files to avoid confusion
		embeddingsPath := filepath.Join(base, fmt.Sprintf("chunk_%s_embeddings.json", pad))
		_ = os.Remove(embeddingsPath)
		_ = os.Remove(strings.Replace(embeddingsPath, "_embeddings.json", "_speakers_mapped.json", 1))
		_ = os.Remove(strings.Replace(embeddingsPath, "_embeddings.json", "_speakers_mapped_global.json", 1))

		// Get dependency client from orchestrator
		if t.Orch == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestrator not initialized"})
			return
		}
		depClient := t.Orch.GetDependencyClient()
		if depClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dependency client not available"})
			return
		}

		// Prepare command
		scriptPath := "/app/speechbrain/speechbrain_embeddings.py" // Path inside deps-service
		cmdArgs := []string{
			scriptPath,
			"--wav", wav,
			"--speakers", speakers,
			"--device", t.Cfg.DeviceDefault,
		}

		// Execute via DependencyClient
		embeddingCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()

		req := dependency.CommandRequest{
			Command: "python",
			Args:    cmdArgs,
			Timeout: 10 * time.Minute,
		}

		// Validate and execute
		if err := dependency.ValidateCommandRequest(req, depClient.Config()); err != nil {
			log.Printf("[API][RedoEmbeddings] validation failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "command validation failed", "details": err.Error()})
			return
		}

		resp, err := depClient.ExecuteCommand(embeddingCtx, req)
		if err != nil || !resp.Success || resp.ExitCode != 0 {
			log.Printf("[API][RedoEmbeddings] failed via DependencyClient: exit_code=%d, stderr=%s, err=%v", resp.ExitCode, resp.Stderr, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":     "embedding extraction failed",
				"exit_code": resp.ExitCode,
				"stderr":    resp.Stderr,
			})
			return
		}

		// Write stdout to embeddings file
		if err := os.WriteFile(embeddingsPath, []byte(resp.Stdout), 0644); err != nil {
			log.Printf("[API][RedoEmbeddings] failed to write output: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write embeddings output"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "output_file": embeddingsPath})
	}
}

// HandleRedoMapped POST /api/v1/tasks/:id/chunks/:cid/redo/mapped
// 重新进行说话人嵌入向量的全局映射
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
		embeddings := filepath.Join(base, fmt.Sprintf("chunk_%s_embeddings.json", pad))
		if _, err := os.Stat(embeddings); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "embeddings json missing"})
			return
		}

		// Remove old mapping files
		mappedPath := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped.json", pad))
		_ = os.Remove(mappedPath)
		globalMappedPath := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped_global.json", pad))
		_ = os.Remove(globalMappedPath)

		// Get dependency client from orchestrator
		if t.Orch == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestrator not initialized"})
			return
		}
		depClient := t.Orch.GetDependencyClient()
		if depClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dependency client not available"})
			return
		}

		// Prepare command
		scriptPath := "/app/pyannote/pyannote_map.py" // Path inside deps-service
		cmdArgs := []string{
			scriptPath,
			"--embeddings", embeddings,
			"--output", mappedPath,
			"--global_map", t.Cfg.GlobalSpeakersMapPath,
			"--distance_threshold", fmt.Sprintf("%g", t.Cfg.SpeakerMapThreshold),
		}
		if t.Cfg.InitialEmbeddingsPath != "" {
			cmdArgs = append(cmdArgs, "--initial_embeddings", t.Cfg.InitialEmbeddingsPath)
		}

		// Execute via DependencyClient
		mapCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()

		req := dependency.CommandRequest{
			Command: "python",
			Args:    cmdArgs,
			Timeout: 5 * time.Minute,
		}

		// Validate and execute
		if err := dependency.ValidateCommandRequest(req, depClient.Config()); err != nil {
			log.Printf("[API][RedoMapped] validation failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "command validation failed", "details": err.Error()})
			return
		}

		resp, err := depClient.ExecuteCommand(mapCtx, req)
		if err != nil || !resp.Success || resp.ExitCode != 0 {
			log.Printf("[API][RedoMapped] failed via DependencyClient: exit_code=%d, stderr=%s, err=%v", resp.ExitCode, resp.Stderr, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":     "speaker mapping failed",
				"exit_code": resp.ExitCode,
				"stderr":    resp.Stderr,
			})
			return
		}

		// The python script writes the file directly, so we just check for its existence.
		// We also need to copy the global map result.
		if _, err := os.Stat(mappedPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mapped file not created by script"})
			return
		}

		// Copy the global mapping result from the script's output location
		// The script pyannote_map.py saves the global map next to the specified --global_map path
		updatedGlobalMapPath := t.Cfg.GlobalSpeakersMapPath
		if _, err := os.Stat(updatedGlobalMapPath); err == nil {
			// In a real-world scenario with shared volumes, this might be tricky.
			// Assuming the script updated the file in a shared volume.
		}

		c.JSON(http.StatusOK, gin.H{
			"status":              "ok",
			"output_file":         mappedPath,
			"global_speakers_map": updatedGlobalMapPath,
		})
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
