package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultWindowHours = 48

type Config struct {
	ReferenceTimezone string       `yaml:"reference_timezone"`
	WindowHours       int          `yaml:"window_hours"`
	Team              []TeamMember `yaml:"team"`
}

type TeamMember struct {
	Name         string         `yaml:"name"`
	Timezone     string         `yaml:"timezone"`
	WorkingHours []WorkingHours `yaml:"working_hours"`
}

type WorkingHours struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

func DefaultPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return "zoneaware.yaml"
		}

		return filepath.Join(home, ".config", "zoneaware.yaml")
	}

	return filepath.Join(configDir, "zoneaware.yaml")
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("config file not found at %s; create one from examples/zoneaware.yaml or pass -config", path)
		}

		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	applyDefaults(&cfg)
	if err := validate(cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close config encoder: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), "zoneaware-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}

	tempPath := tempFile.Name()
	if _, err := tempFile.Write(buffer.Bytes()); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp config: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace config: %w", err)
	}

	return nil
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	applyDefaults(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.ReferenceTimezone) == "" {
		cfg.ReferenceTimezone = defaultReferenceTimezone()
	}

	if cfg.WindowHours <= 0 {
		cfg.WindowHours = DefaultWindowHours
	}
}

func validate(cfg Config) error {
	if _, err := time.LoadLocation(cfg.ReferenceTimezone); err != nil {
		return fmt.Errorf("invalid reference_timezone %q: %w", cfg.ReferenceTimezone, err)
	}

	if len(cfg.Team) == 0 {
		return errors.New("config must include at least one team member")
	}

	seenNames := make(map[string]struct{}, len(cfg.Team))
	for i, member := range cfg.Team {
		name := strings.TrimSpace(member.Name)
		if name == "" {
			return fmt.Errorf("team[%d].name is required", i)
		}

		nameKey := strings.ToLower(name)
		if _, exists := seenNames[nameKey]; exists {
			return fmt.Errorf("duplicate team member name %q", member.Name)
		}
		seenNames[nameKey] = struct{}{}

		if strings.TrimSpace(member.Timezone) == "" {
			return fmt.Errorf("team[%d].timezone is required", i)
		}

		if _, err := time.LoadLocation(member.Timezone); err != nil {
			return fmt.Errorf("invalid timezone for %q: %w", member.Name, err)
		}

		for j, hours := range member.WorkingHours {
			start, err := ParseClock(hours.Start)
			if err != nil {
				return fmt.Errorf("team member %q working_hours[%d].start: %w", member.Name, j, err)
			}

			end, err := ParseClock(hours.End)
			if err != nil {
				return fmt.Errorf("team member %q working_hours[%d].end: %w", member.Name, j, err)
			}

			if start == end {
				return fmt.Errorf("team member %q working_hours[%d] cannot have the same start and end", member.Name, j)
			}
		}
	}

	return nil
}

func EffectiveWindowHours(cfg Config) int {
	if cfg.WindowHours < DefaultWindowHours {
		return DefaultWindowHours
	}

	return cfg.WindowHours
}

func defaultReferenceTimezone() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}

	locationName := time.Now().Location().String()
	if strings.TrimSpace(locationName) != "" {
		return locationName
	}

	return "UTC"
}

func ParseClock(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "24:00" {
		return 24 * 60, nil
	}

	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("must be in HH:MM format")
	}

	return parsed.Hour()*60 + parsed.Minute(), nil
}
