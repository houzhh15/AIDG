package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/dependency"
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
		// Determine merge-segments binary path: prefer local bin/merge-segments for development
		mergeCmd := "merge-segments"
		if _, err := os.Stat("./bin/merge-segments"); err == nil {
			mergeCmd = "./bin/merge-segments"
		} else if _, err := os.Stat("go-whisper/merge-segments"); err == nil {
			mergeCmd = "go-whisper/merge-segments"
		}

		if _, err := os.Stat(mergeCmd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "merge-segments binary not found",
				"detail": fmt.Sprintf("expected at %s", mergeCmd),
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
		args := []string{mergeCmd, "--segments-file", seg, "--speaker-file", spk}
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
			log.Println("[API][RedoSpeakers] orchestrator is nil")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestrator not initialized"})
			return
		}
		depClient := t.Orch.GetDependencyClient()
		if depClient == nil {
			log.Println("[API][RedoSpeakers] dependency client is nil")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dependency client not available"})
			return
		}

		// Note: Both aidg-unified and aidg-deps-service use /app/data after volume mount fix,
		// so paths (wav, speakersPath) can be used directly without transformation

		// Execute diarization via DependencyClient high-level API
		// Use 30 minutes timeout (same as queue processing) to allow model download on first run
		diarizationCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
		defer cancel()

		// Prepare diarization options
		opts := &dependency.DiarizationOptions{
			Device:        t.Cfg.DeviceDefault,
			EnableOffline: t.Cfg.EnableOffline,
			// Note: HFToken will be read from environment in deps-service
			// No need to pass it here for security reasons
		}

		// Use RunDiarization high-level API which handles script path internally
		err = depClient.RunDiarization(diarizationCtx, wav, speakersPath, opts)
		if err != nil {
			log.Printf("[API][RedoSpeakers] diarization failed via DependencyClient: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "diarization failed",
				"details": err.Error(),
			})
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

		// Note: Both aidg-unified and aidg-deps-service use /app/data after volume mount fix,
		// so paths (wav, speakers, embeddingsPath) can be used directly without transformation

		// Execute embedding extraction via DependencyClient high-level API
		// Use 30 minutes timeout (same as queue processing) to allow model download on first run
		embeddingCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
		defer cancel()

		// Prepare embedding options
		opts := &dependency.EmbeddingOptions{
			Device:             t.Cfg.EmbeddingDeviceDefault,
			EnableOffline:      t.Cfg.EnableOffline,
			Threshold:          t.Cfg.EmbeddingThreshold,
			AutoLowerThreshold: true, // Always enable auto-lowering for API calls
			AutoLowerMin:       t.Cfg.EmbeddingAutoLowerMin,
			AutoLowerStep:      t.Cfg.EmbeddingAutoLowerStep,
			// Note: HFToken will be read from environment in deps-service
			// No need to pass it here for security reasons
		}

		// Use GenerateEmbeddings high-level API which handles script path internally
		err = depClient.GenerateEmbeddings(embeddingCtx, wav, speakers, embeddingsPath, opts)
		if err != nil {
			log.Printf("[API][RedoEmbeddings] embedding extraction failed via DependencyClient: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "embedding extraction failed",
				"details": err.Error(),
			})
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

		speakers := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers.json", pad))
		if _, err := os.Stat(speakers); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "speakers json missing"})
			return
		}

		// Remove old mapping files
		mappedPath := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped.json", pad))
		_ = os.Remove(mappedPath)
		globalMappedPath := filepath.Join(base, fmt.Sprintf("chunk_%s_speakers_mapped_global.json", pad))
		_ = os.Remove(globalMappedPath)

		// Apply local mapping first, then global mapping (following mergeWorker pattern)
		mapped := speakers
		if p, err := applyLocalMapping(speakers, embeddings); err == nil && p != "" {
			mapped = p
			log.Printf("[API][RedoMapped] chunk %04d local mapping -> %s", n, filepath.Base(mapped))
		}
		if p, err := applyGlobalMapping(mapped, embeddings); err == nil && p != "" {
			mapped = p
			log.Printf("[API][RedoMapped] chunk %04d global mapping -> %s", n, filepath.Base(mapped))
		}

		// Check if mapping actually produced a new file
		if mapped == speakers {
			// No mapping was applied, create a copy of the original speakers file
			mapped = mappedPath
			if err := copyFile(speakers, mapped); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy speakers file", "details": err.Error()})
				return
			}
		}

		// Verify the final mapped file exists
		if _, err := os.Stat(mapped); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "mapped file not created"})
			return
		}

		// Return success with file paths
		c.JSON(http.StatusOK, gin.H{
			"status":              "ok",
			"output_file":         mapped,
			"global_speakers_map": t.Cfg.GlobalSpeakersMapPath,
		})
	}
}

// copyFile copies a file from src to dst, preserving contents. Caller is
// responsible for handling any cleanup of the source file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

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
