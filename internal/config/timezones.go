package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var zoneTabCandidates = []string{
	"/usr/share/zoneinfo/zone1970.tab",
	"/usr/share/lib/zoneinfo/zone1970.tab",
}

func AvailableTimezones() ([]string, error) {
	for _, path := range zoneTabCandidates {
		timezones, err := parseZoneTab(path)
		if err == nil && len(timezones) > 0 {
			return timezones, nil
		}
	}

	return nil, fmt.Errorf("could not load timezone list from %s", strings.Join(zoneTabCandidates, ", "))
}

func parseZoneTab(path string) ([]string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	seen := make(map[string]bool)
	timezones := make([]string, 0, 512)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		zone := fields[2]
		if seen[zone] {
			continue
		}
		seen[zone] = true
		timezones = append(timezones, zone)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Strings(timezones)
	return timezones, nil
}
