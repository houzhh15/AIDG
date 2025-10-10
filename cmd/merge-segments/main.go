package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// --- Local data structures (no external dependencies) ---

// Timestamp represents a time duration in milliseconds
type Timestamp time.Duration

// MarshalJSON encodes Timestamp as milliseconds
func (t Timestamp) MarshalJSON() ([]byte, error) {
	ms := time.Duration(t).Milliseconds()
	return json.Marshal(ms)
}

// UnmarshalJSON decodes milliseconds into Timestamp
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var ms int64
	if err := json.Unmarshal(data, &ms); err != nil {
		return err
	}
	*t = Timestamp(time.Duration(ms) * time.Millisecond)
	return nil
}

// Segment represents an ASR transcription segment
type Segment struct {
	Id       int32     `json:"id"`
	Start    Timestamp `json:"start"`
	End      Timestamp `json:"end"`
	Text     string    `json:"text"`
	Speaker  string    `json:"speaker,omitempty"`
	Language string    `json:"language,omitempty"`
}

// Transcription represents a full transcription with segments
type Transcription struct {
	Text     string     `json:"text"`
	Language string     `json:"language,omitempty"`
	Segments []*Segment `json:"segments"`
}

// --- Output formatting functions ---

// WriteSegmentText writes segment in text format: "[HH:MM:SS.mmm --> HH:MM:SS.mmm] [Speaker] Text"
func WriteSegmentText(w io.Writer, s *Segment) {
	startStr := formatTimestamp(s.Start)
	endStr := formatTimestamp(s.End)
	speaker := ""
	if s.Speaker != "" {
		speaker = fmt.Sprintf(" [%s]", s.Speaker)
	}
	fmt.Fprintf(w, "[%s --> %s]%s %s", startStr, endStr, speaker, s.Text)
}

// WriteSegmentSrt writes segment in SRT format
func WriteSegmentSrt(w io.Writer, s *Segment) {
	fmt.Fprintf(w, "%d\n", s.Id+1)
	startStr := formatTimestampSrt(s.Start)
	endStr := formatTimestampSrt(s.End)
	fmt.Fprintf(w, "%s --> %s\n", startStr, endStr)
	fmt.Fprintf(w, "%s\n", s.Text)
}

// WriteSegmentVtt writes segment in WebVTT format
func WriteSegmentVtt(w io.Writer, s *Segment) {
	startStr := formatTimestampVtt(s.Start)
	endStr := formatTimestampVtt(s.End)
	fmt.Fprintf(w, "%s --> %s\n", startStr, endStr)
	fmt.Fprintf(w, "%s\n", s.Text)
}

// formatTimestamp formats as HH:MM:SS.mmm
func formatTimestamp(t Timestamp) string {
	d := time.Duration(t)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// formatTimestampSrt formats as HH:MM:SS,mmm (SRT uses comma)
func formatTimestampSrt(t Timestamp) string {
	d := time.Duration(t)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// formatTimestampVtt formats as HH:MM:SS.mmm (WebVTT uses dot)
func formatTimestampVtt(t Timestamp) string {
	return formatTimestamp(t)
}

// Simple tool: merge ASR segments with diarization speakers and output
// Usage:
//   merge-segments -segments-file <segments.(json|srt|vtt)> [-speaker-file diarization.json] [-format text|json|srt|vtt]

func main() {
	var segmentsFile string
	var speakerFile string
	var format string
	flag.Usage = func() {
		exe := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s -segments-file <segments.(json|srt|vtt|ndjson)> -speaker-file <diarization.json> [-format text|json|verbose_json|srt|vtt]\n\n", exe)
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}
	flag.StringVar(&segmentsFile, "segments-file", "", "Path to ASR segments file (json/srt/vtt/ndjson)")
	flag.StringVar(&speakerFile, "speaker-file", "", "Path to diarization JSON file (required)")
	flag.StringVar(&format, "format", "text", "Output format: json|verbose_json|text|srt|vtt")
	flag.Parse()

	// Validate required flags
	if segmentsFile == "" || speakerFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	// Validate format
	if !validFormat(format) {
		fmt.Fprintln(os.Stderr, "invalid -format:", format)
		flag.Usage()
		os.Exit(2)
	}

	segs, err := parseSegmentsFromFile(segmentsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read segments:", err)
		os.Exit(1)
	}

	if segs, err = labelSpeakersFromFile(speakerFile, segs); err != nil {
		fmt.Fprintln(os.Stderr, "merge speakers:", err)
		os.Exit(1)
	} else if len(segs) == 0 {
		fmt.Fprintln(os.Stderr, "no segments after speaker merge")
	}

	for i, segment := range segs {
		var out bytes.Buffer
		switch format {
		case "json", "verbose_json":
			fmt.Println(segment)
		case "srt":
			WriteSegmentSrt(&out, segment)
			fmt.Println(out.String())
		case "vtt":
			if i == 0 {
				fmt.Println("WEBVTT")
				fmt.Println()
			}
			WriteSegmentVtt(&out, segment)
			fmt.Println(out.String())
		case "text":
			fallthrough
		default:
			WriteSegmentText(&out, segment)
			fmt.Println(out.String())
		}
	}
}

func validFormat(f string) bool {
	switch f {
	case "json", "verbose_json", "text", "srt", "vtt":
		return true
	default:
		return false
	}
}

// --- Types and helpers copied from whisper CLI implementation ---

type diarizeSegment struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker string  `json:"speaker"`
}

// labelSpeakersFromFile reads a diarization JSON file and merges into segments.
func labelSpeakersFromFile(filePath string, segs []*Segment) ([]*Segment, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Segments []diarizeSegment `json:"segments"`
		Error    string           `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		// try to extract JSON if file contains extra logs
		if jb, jerr := extractJSONWithSegments(data); jerr == nil {
			if err2 := json.Unmarshal(jb, &payload); err2 != nil {
				return nil, err2
			}
		} else {
			return nil, err
		}
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("pyannote error: %s", payload.Error)
	}
	if len(payload.Segments) == 0 {
		return segs, nil
	}
	// Merge: improved algorithm with ASR overlap ratio threshold and speaker coverage optimization
	const minASROverlapRatio = 0.2 // ASR segment must have at least 70% overlap

	for _, s := range segs {
		asrStart := float64(time.Duration(s.Start).Seconds())
		asrEnd := float64(time.Duration(s.End).Seconds())
		asrDuration := asrEnd - asrStart

		if asrDuration <= 0 {
			continue // Skip invalid segments
		}

		type SpeakerCandidate struct {
			Speaker      string
			Overlap      float64
			ASRRatio     float64
			SpeakerRatio float64
		}

		var candidates []SpeakerCandidate

		// Step 1: Filter candidates with ASR overlap ratio >= 0.7
		for _, d := range payload.Segments {
			start := maxFloat(asrStart, d.Start)
			end := minFloat(asrEnd, d.End)
			if end > start {
				overlap := end - start
				asrRatio := overlap / asrDuration

				if asrRatio >= minASROverlapRatio {
					spkDuration := d.End - d.Start
					spkRatio := 0.0
					if spkDuration > 0 {
						spkRatio = overlap / spkDuration
					}

					candidates = append(candidates, SpeakerCandidate{
						Speaker:      d.Speaker,
						Overlap:      overlap,
						ASRRatio:     asrRatio,
						SpeakerRatio: spkRatio,
					})
				}
			}
		}

		// Step 2: Select candidate with highest speaker coverage ratio
		if len(candidates) > 0 {
			var bestCandidate SpeakerCandidate
			for _, candidate := range candidates {
				if candidate.SpeakerRatio > bestCandidate.SpeakerRatio {
					bestCandidate = candidate
				}
			}
			s.Speaker = bestCandidate.Speaker
		}
		// If no candidates meet the threshold, leave speaker empty (no assignment)
	}
	return segs, nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// parseSegmentsFromFile parses ASR segments from json (Transcription with segments, array, or NDJSON), srt, or vtt.
func parseSegmentsFromFile(filePath string) ([]*Segment, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		// Try Transcription {segments:[]}
		var tr Transcription
		if err := json.Unmarshal(data, &tr); err == nil && len(tr.Segments) > 0 {
			return tr.Segments, nil
		}
		// Try array of segments
		var arr []*Segment
		if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
			return arr, nil
		}
		// Try NDJSON lines (each line a Segment)
		var res []*Segment
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var s Segment
			if err := json.Unmarshal([]byte(line), &s); err == nil && (s.Text != "" || s.End > 0) {
				res = append(res, &s)
			}
		}
		if len(res) > 0 {
			return res, nil
		}
		// Try concatenated multi-line JSON objects by brace matching
		if objs := splitConcatenatedJSONObjects(data); len(objs) > 0 {
			var out []*Segment
			for _, obj := range objs {
				// Try Segment first
				var s Segment
				if err := json.Unmarshal(obj, &s); err == nil && (s.Text != "" || s.End > 0) {
					out = append(out, &s)
					continue
				}
				// Try Transcription
				var tr Transcription
				if err := json.Unmarshal(obj, &tr); err == nil && len(tr.Segments) > 0 {
					out = append(out, tr.Segments...)
					continue
				}
			}
			if len(out) > 0 {
				return out, nil
			}
		}
		return nil, errors.New("no segments found in JSON")
	case ".srt":
		return parseSRT(f)
	case ".vtt":
		return parseVTT(f)
	default:
		// Try best-effort as SRT then VTT
		if segs, err := parseSRT(f); err == nil {
			return segs, nil
		}
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			if segs, err := parseVTT(f); err == nil {
				return segs, nil
			}
		}
		return nil, fmt.Errorf("unsupported segments file: %s", filePath)
	}
}

var (
	reSrtTime = regexp.MustCompile(`^(\d\d):(\d\d):(\d\d),(\d\d\d)\s+-->\s+(\d\d):(\d\d):(\d\d),(\d\d\d)`)
	reVttTime = regexp.MustCompile(`^(\d\d):(\d\d):(\d\d)\.(\d\d\d)\s+-->\s+(\d\d):(\d\d):(\d\d)\.(\d\d\d)`)
)

func parseSRT(r io.Reader) ([]*Segment, error) {
	scanner := bufio.NewScanner(r)
	var segs []*Segment
	var idx int32
	for {
		// consume optional index line
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// expect time line
		if !scanner.Scan() {
			break
		}
		tline := strings.TrimSpace(scanner.Text())
		m := reSrtTime.FindStringSubmatch(tline)
		if m == nil {
			continue
		}
		start := hmsmsToDur(m[1], m[2], m[3], m[4])
		end := hmsmsToDur(m[5], m[6], m[7], m[8])
		// read text lines until blank
		var b strings.Builder
		for scanner.Scan() {
			l := scanner.Text()
			if strings.TrimSpace(l) == "" {
				break
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(l)
		}
		segs = append(segs, &Segment{Id: idx, Start: Timestamp(start), End: Timestamp(end), Text: b.String()})
		idx++
	}
	if len(segs) == 0 {
		return nil, errors.New("no segments in SRT")
	}
	return segs, nil
}

func parseVTT(r io.Reader) ([]*Segment, error) {
	scanner := bufio.NewScanner(r)
	var segs []*Segment
	var idx int32
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			if strings.HasPrefix(strings.ToUpper(line), "WEBVTT") {
				// skip header
				continue
			}
		}
		if line == "" {
			continue
		}
		m := reVttTime.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start := hmsmsToDur(m[1], m[2], m[3], m[4])
		end := hmsmsToDur(m[5], m[6], m[7], m[8])
		var b strings.Builder
		for scanner.Scan() {
			l := scanner.Text()
			if strings.TrimSpace(l) == "" {
				break
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(l)
		}
		segs = append(segs, &Segment{Id: idx, Start: Timestamp(start), End: Timestamp(end), Text: b.String()})
		idx++
	}
	if len(segs) == 0 {
		return nil, errors.New("no segments in VTT")
	}
	return segs, nil
}

func hmsmsToDur(hh, mm, ss, ms string) time.Duration {
	h := atoi(hh)
	m := atoi(mm)
	s := atoi(ss)
	msI := atoi(ms)
	return time.Duration(((h*3600+m*60+s)*1000 + msI)) * time.Millisecond
}

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i] - '0'
		if c <= 9 {
			n = n*10 + int(c)
		}
	}
	return n
}

// splitConcatenatedJSONObjects scans a byte slice and extracts consecutive top-level JSON objects.
func splitConcatenatedJSONObjects(b []byte) [][]byte {
	var out [][]byte
	depth := 0
	inString := false
	escaped := false
	start := -1
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					out = append(out, b[start:i+1])
					start = -1
				}
			}
		}
	}
	return out
}

// extractJSONWithSegments tries to locate a JSON object containing a "segments" field
// within potentially noisy mixed output (warnings, logs, etc.).
func extractJSONWithSegments(out []byte) ([]byte, error) {
	// Prefer the last occurrence of a JSON object starting with {"segments"
	key := []byte("{\"segments\"")
	start := bytes.LastIndex(out, key)
	if start < 0 {
		// fallback: last '{'
		start = bytes.LastIndex(out, []byte("{"))
		if start < 0 {
			return nil, errors.New("no JSON object found")
		}
	}
	// Scan forward to match braces
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(out); i++ {
		c := out[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return out[start : i+1], nil
			}
		}
	}
	return nil, errors.New("unterminated JSON object")
}
