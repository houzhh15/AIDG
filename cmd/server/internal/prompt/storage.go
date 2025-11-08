package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PromptStorage handles file system persistence for prompts
type PromptStorage struct {
	baseDir string // Base directory for data storage
}

// NewPromptStorage creates a new storage instance
func NewPromptStorage(baseDir string) *PromptStorage {
	return &PromptStorage{baseDir: baseDir}
}

// Save persists a prompt to the file system (JSON + MD)
func (s *PromptStorage) Save(prompt *Prompt) error {
	// Validate prompt_id to prevent path traversal
	if err := s.validatePromptID(prompt.PromptID); err != nil {
		return fmt.Errorf("invalid prompt_id: %w", err)
	}

	// Determine target directory based on scope
	targetDir, err := s.getTargetDir(prompt)
	if err != nil {
		return fmt.Errorf("failed to determine target directory: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Save JSON file
	jsonPath := filepath.Join(targetDir, fmt.Sprintf("%s.json", prompt.PromptID))
	if err := s.saveJSON(jsonPath, prompt); err != nil {
		return fmt.Errorf("failed to save JSON: %w", err)
	}

	// Generate and save MD file
	if err := s.GenerateMDFile(prompt); err != nil {
		return fmt.Errorf("failed to generate MD file: %w", err)
	}

	return nil
}

// Load retrieves a single prompt from file system
func (s *PromptStorage) Load(promptID string) (*Prompt, error) {
	// Validate prompt_id
	if err := s.validatePromptID(promptID); err != nil {
		return nil, fmt.Errorf("invalid prompt_id: %w", err)
	}

	// Search in all possible locations
	searchDirs := s.getSearchDirs()
	for _, dir := range searchDirs {
		jsonPath := filepath.Join(dir, fmt.Sprintf("%s.json", promptID))
		if _, err := os.Stat(jsonPath); err == nil {
			return s.loadJSON(jsonPath)
		}
	}

	return nil, fmt.Errorf("prompt not found: %s", promptID)
}

// LoadAll loads prompts from specified scope with optional filters
func (s *PromptStorage) LoadAll(scope string, filter map[string]string) ([]*Prompt, error) {
	var prompts []*Prompt

	// Determine search directories based on scope
	searchDirs, err := s.getScopedDirs(scope, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get scoped directories: %w", err)
	}

	// Scan each directory for JSON files
	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			jsonPath := filepath.Join(dir, entry.Name())
			prompt, err := s.loadJSON(jsonPath)
			if err != nil {
				// Log error but continue processing
				fmt.Printf("[WARN] Failed to load %s: %v\n", jsonPath, err)
				continue
			}

			// Apply filters
			if s.matchesFilter(prompt, filter) {
				prompts = append(prompts, prompt)
			}
		}
	}

	return prompts, nil
}

// Delete removes a prompt's JSON and MD files
func (s *PromptStorage) Delete(promptID string) error {
	// Validate prompt_id
	if err := s.validatePromptID(promptID); err != nil {
		return fmt.Errorf("invalid prompt_id: %w", err)
	}

	// Search and delete in all possible locations
	searchDirs := s.getSearchDirs()
	found := false
	for _, dir := range searchDirs {
		jsonPath := filepath.Join(dir, fmt.Sprintf("%s.json", promptID))
		mdPath := filepath.Join(dir, fmt.Sprintf("%s.prompt.md", promptID))

		// Delete JSON file
		if err := os.Remove(jsonPath); err == nil {
			found = true
		}

		// Delete MD file (ignore errors if it doesn't exist)
		os.Remove(mdPath)
	}

	if !found {
		return fmt.Errorf("prompt not found: %s", promptID)
	}

	return nil
}

// GenerateMDFile creates a .prompt.md file with YAML frontmatter
func (s *PromptStorage) GenerateMDFile(prompt *Prompt) error {
	targetDir, err := s.getTargetDir(prompt)
	if err != nil {
		return fmt.Errorf("failed to determine target directory: %w", err)
	}

	mdPath := filepath.Join(targetDir, fmt.Sprintf("%s.prompt.md", prompt.PromptID))

	// Prepare frontmatter
	frontmatter := map[string]interface{}{
		"name":        prompt.Name,
		"description": prompt.Description,
	}

	if len(prompt.Arguments) > 0 {
		args := make([]map[string]interface{}, len(prompt.Arguments))
		for i, arg := range prompt.Arguments {
			args[i] = map[string]interface{}{
				"name":        arg.Name,
				"description": arg.Description,
				"required":    arg.Required,
			}
		}
		frontmatter["arguments"] = args
	}

	// Marshal frontmatter to YAML
	yamlData, err := yaml.Marshal(frontmatter)
	if err != nil {
		return fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	// Combine frontmatter and content
	content := fmt.Sprintf("---\n%s---\n\n%s", string(yamlData), prompt.Content)

	// Write to file
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write MD file: %w", err)
	}

	return nil
}

// Helper methods

func (s *PromptStorage) validatePromptID(promptID string) error {
	// Prevent path traversal
	if strings.Contains(promptID, "..") || strings.Contains(promptID, "/") || strings.Contains(promptID, "\\") {
		return fmt.Errorf("invalid characters in prompt_id")
	}
	if promptID == "" {
		return fmt.Errorf("prompt_id cannot be empty")
	}
	return nil
}

func (s *PromptStorage) getTargetDir(prompt *Prompt) (string, error) {
	switch prompt.Scope {
	case ScopeGlobal:
		return filepath.Join(s.baseDir, "prompts", "global"), nil
	case ScopeProject:
		if prompt.ProjectID == "" {
			return "", fmt.Errorf("project_id required for project scope")
		}
		return filepath.Join(s.baseDir, "projects", prompt.ProjectID, "prompts"), nil
	case ScopePersonal:
		if prompt.Owner == "" {
			return "", fmt.Errorf("owner required for personal scope")
		}
		return filepath.Join(s.baseDir, "users", prompt.Owner, "prompts"), nil
	default:
		return "", fmt.Errorf("invalid scope: %s", prompt.Scope)
	}
}

func (s *PromptStorage) getSearchDirs() []string {
	var dirs []string

	// 1. Global prompts
	globalDir := filepath.Join(s.baseDir, "prompts", "global")
	dirs = append(dirs, globalDir)

	// 2. Project prompts - scan all projects
	projectsDir := filepath.Join(s.baseDir, "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				promptsDir := filepath.Join(projectsDir, entry.Name(), "prompts")
				if _, err := os.Stat(promptsDir); err == nil {
					dirs = append(dirs, promptsDir)
				}
			}
		}
	}

	// 3. Personal prompts - scan all users
	usersDir := filepath.Join(s.baseDir, "users")
	if entries, err := os.ReadDir(usersDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				promptsDir := filepath.Join(usersDir, entry.Name(), "prompts")
				if _, err := os.Stat(promptsDir); err == nil {
					dirs = append(dirs, promptsDir)
				}
			}
		}
	}

	return dirs
}

func (s *PromptStorage) getScopedDirs(scope string, filter map[string]string) ([]string, error) {
	var dirs []string

	switch scope {
	case ScopeGlobal:
		dirs = append(dirs, filepath.Join(s.baseDir, "prompts", "global"))
	case ScopeProject:
		if projectID, ok := filter["project_id"]; ok {
			dirs = append(dirs, filepath.Join(s.baseDir, "projects", projectID, "prompts"))
		}
	case ScopePersonal:
		if owner, ok := filter["owner"]; ok {
			dirs = append(dirs, filepath.Join(s.baseDir, "users", owner, "prompts"))
		}
	case "":
		// Load all scopes
		dirs = append(dirs, filepath.Join(s.baseDir, "prompts", "global"))
		// Note: For full scan, would need to iterate projects/users
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}

	return dirs, nil
}

func (s *PromptStorage) saveJSON(path string, prompt *Prompt) error {
	data, err := json.MarshalIndent(prompt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (s *PromptStorage) loadJSON(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var prompt Prompt
	if err := json.Unmarshal(data, &prompt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &prompt, nil
}

func (s *PromptStorage) matchesFilter(prompt *Prompt, filter map[string]string) bool {
	if visibility, ok := filter["visibility"]; ok {
		if prompt.Visibility != visibility {
			return false
		}
	}

	if owner, ok := filter["owner"]; ok {
		if prompt.Owner != owner {
			return false
		}
	}

	return true
}
