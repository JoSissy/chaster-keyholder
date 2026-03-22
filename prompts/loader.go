// Package prompts loads and compiles the YAML prompt configuration at startup.
// All AI prompt strings and numeric thresholds live in prompts/prompts.yaml.
// Call Load() once from main.go, then pass the *Loader to ai.NewClient().
package prompts

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed prompts.yaml
var defaultPromptsYAML []byte

// Models holds the Groq model identifiers.
type Models struct {
	Text   string `yaml:"text"`
	Vision string `yaml:"vision"`
}

// Thresholds holds numeric business-logic limits referenced in prompts.
// Changing these here avoids hunting down magic numbers in keyholder.go.
type Thresholds struct {
	PilloryMinMinutes      int `yaml:"pillory_min_minutes"`
	FreezeDefaultMinutes   int `yaml:"freeze_default_minutes"`
	HidetimeDefaultMinutes int `yaml:"hidetime_default_minutes"`
	AddtimeDefaultMinutes  int `yaml:"addtime_default_minutes"`
	LockDurationDefault    int `yaml:"lock_duration_default_hours"`
	NegotiateMaxRemove     int `yaml:"negotiate_max_remove_hours"`
	NegotiateMaxPenalty    int `yaml:"negotiate_max_penalty_hours"`
}

// SystemPrompts holds static system-role prompt strings (no variable interpolation).
type SystemPrompts struct {
	Locked               string `yaml:"locked"`
	Free                 string `yaml:"free"`
	TaskExplanation      string `yaml:"task_explanation"`
	VerifyTaskPhoto      string `yaml:"verify_task_photo"`
	VerifyLockPhoto      string `yaml:"verify_lock_photo"`
	ClassifyIntent       string `yaml:"classify_intent"`
	ExtractContractRules string `yaml:"extract_contract_rules"`
	DescribeToy          string `yaml:"describe_toy"`
	DescribeClothing     string `yaml:"describe_clothing"`
	PilloryReason        string `yaml:"pillory_reason"`
	ChasterTask          string `yaml:"chaster_task"`
}

// ObedienceCtxs holds the per-level context suffix strings appended to prompts.
// Each value starts with a space so it can be appended directly to another string.
type ObedienceCtxs struct {
	L0 string `yaml:"0"`
	L1 string `yaml:"1"`
	L2 string `yaml:"2"`
	L3 string `yaml:"3"`
	L4 string `yaml:"4"`
}

// rawConfig is the top-level YAML structure, used only during loading.
type rawConfig struct {
	Models    Models              `yaml:"models"`
	Thresholds Thresholds         `yaml:"thresholds"`
	System    SystemPrompts       `yaml:"system"`
	Obedience ObedienceCtxs      `yaml:"obedience_context"`
	Suffixes  map[string]string   `yaml:"suffixes"`
	Prompts   map[string]string   `yaml:"prompts"`
	Lists     map[string][]string `yaml:"lists"`
}

// Loader is the compiled, ready-to-use prompt configuration.
// All templates are pre-compiled at startup so errors surface immediately.
type Loader struct {
	Models     Models
	Thresholds Thresholds
	System     SystemPrompts
	Obedience  ObedienceCtxs
	Lists      map[string][]string // e.g. Lists["random_message_styles"]

	raw  map[string]string // raw template strings (suffixes + prompts merged)
	tmpl map[string]*template.Template
}

// BuildSystemPrompt returns the correct system prompt for the lock state.
func (l *Loader) BuildSystemPrompt(locked bool) string {
	if locked {
		return l.System.Locked
	}
	return l.System.Free
}

// ObedienceCtx returns the obedience context suffix string for level 0–4.
// The returned string begins with a space for direct concatenation.
func (l *Loader) ObedienceCtx(level int) string {
	switch level {
	case 4:
		return l.Obedience.L4
	case 3:
		return l.Obedience.L3
	case 2:
		return l.Obedience.L2
	case 1:
		return l.Obedience.L1
	default:
		return l.Obedience.L0
	}
}

// Get returns the raw (un-rendered) template string for the given key.
// Useful for suffixes appended to base system prompts before further rendering.
func (l *Loader) Get(key string) string {
	return l.raw[key]
}

// Render executes the named template with data and returns the resulting string.
// data should be a map[string]any or a struct with exported fields matching
// the {{.VarName}} placeholders in the YAML template.
// Missing map keys silently produce empty strings (missingkey=zero).
func (l *Loader) Render(key string, data any) (string, error) {
	t, ok := l.tmpl[key]
	if !ok {
		return "", fmt.Errorf("prompts: template %q not found", key)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompts: render %q: %w", key, err)
	}
	return buf.String(), nil
}

// MustRender is like Render but logs on error and returns the raw template
// text as a fallback rather than crashing. Use in contexts where the caller
// cannot propagate errors (e.g. already inside an error path).
func (l *Loader) MustRender(key string, data any) string {
	s, err := l.Render(key, data)
	if err != nil {
		log.Printf("[prompts] render error: %v — returning raw template", err)
		return l.raw[key]
	}
	return s
}

// Load reads and parses the YAML file at path, compiles all templates,
// and returns a ready-to-use *Loader. Call once at startup from main.go.
// If path is empty or the file does not exist, falls back to the embedded
// prompts.yaml compiled into the binary.
func Load(path string) (*Loader, error) {
	var data []byte
	if path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			log.Printf("⚠️  prompts: no se pudo leer %q (%v) — usando prompts embebidos", path, err)
			data = defaultPromptsYAML
		}
	} else {
		data = defaultPromptsYAML
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("prompts: parse YAML: %w", err)
	}

	// Merge suffixes and prompts into a single flat map so Render/Get
	// work with a single key namespace.
	merged := make(map[string]string, len(raw.Suffixes)+len(raw.Prompts))
	for k, v := range raw.Suffixes {
		merged[k] = v
	}
	for k, v := range raw.Prompts {
		merged[k] = v
	}

	// Pre-compile all templates at startup. Any syntax error in the YAML
	// will cause Load() to fail, surfacing the problem before the bot starts.
	compiled := make(map[string]*template.Template, len(merged))
	for key, text := range merged {
		t, err := template.New(key).Option("missingkey=zero").Parse(text)
		if err != nil {
			return nil, fmt.Errorf("prompts: compile template %q: %w", key, err)
		}
		compiled[key] = t
	}

	return &Loader{
		Models:     raw.Models,
		Thresholds: raw.Thresholds,
		System:     raw.System,
		Obedience:  raw.Obedience,
		Lists:      raw.Lists,
		raw:        merged,
		tmpl:       compiled,
	}, nil
}
