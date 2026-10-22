package main

import (
	"fmt"
	"slices"
	"strings"
)

// configuration mirrors settings_schema in plugin.json.
type configuration struct {
	RelativeHours   string
	EarlyTime       string
	TodayTime       string
	TomorrowTime    string
	MondayTime      string
	DefaultTimezone string
}

// relativeHours reads "1,2" into [1 2]: the "in N hours" quick choices (1–24, at most 4).
// Empty (never set, e.g. after upgrading) means the default "1,2"; "0" hides them.
func (c *configuration) relativeHours() []int {
	raw := strings.TrimSpace(c.RelativeHours)
	if raw == "" {
		raw = "1,2"
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		var h int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &h); err != nil || h < 1 || h > 24 || slices.Contains(out, h) {
			continue
		}
		out = append(out, h)
		if len(out) == 4 {
			break
		}
	}
	return out
}

func (p *Plugin) getConfiguration() *configuration {
	p.configLock.RLock()
	defer p.configLock.RUnlock()

	if p.config == nil {
		return &configuration{}
	}
	return p.config
}

func (p *Plugin) OnConfigurationChange() error {
	cfg := new(configuration)
	if err := p.API.LoadPluginConfiguration(cfg); err != nil {
		return fmt.Errorf("failed to load plugin configuration: %w", err)
	}

	p.configLock.Lock()
	p.config = cfg
	p.configLock.Unlock()

	return nil
}

// parseClock reads a 24-hour "HH:MM" time, falling back to the default when it is empty or invalid.
func parseClock(s string, defHour, defMinute int) (int, int) {
	var h, m int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil {
		return defHour, defMinute
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return defHour, defMinute
	}
	return h, m
}
