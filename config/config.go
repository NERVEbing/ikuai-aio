// Package config reads and validates configuration without process-global state.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/NERVEbing/ikuai-aio/v4/internal/schedule"
)

type Config struct {
	Router         api.Options
	Timezone       *time.Location
	ListenAddress  string
	MetricsEnabled bool
	IPv6           bool
	ScrapeTimeout  time.Duration
	JobTimeout     time.Duration
	SkipStart      bool
	IPObjects      []IPObjectTask
	DomainRules    []DomainRuleTask
}

type Task struct {
	Schedule string   `json:"schedule"`
	Name     string   `json:"name"`
	URLs     []string `json:"urls"`
	Comment  string   `json:"comment"`
}

type IPObjectTask struct{ Task }

type DomainRuleTask struct {
	Task
	Interface string   `json:"interface"`
	Source    []string `json:"source"`
	Priority  int      `json:"priority"`
}

func Load() (Config, error) { return Read(os.Environ()) }

// Read is deterministic: numbered tasks are sorted numerically and duplicate
// resource names are rejected before any network request or mutation.
func Read(environment []string) (Config, error) {
	env := make(map[string]string, len(environment))
	for _, entry := range environment {
		if key, value, ok := strings.Cut(entry, "="); ok {
			env[key] = value
		}
	}
	value := func(key, fallback string) string {
		if v, ok := env[key]; ok {
			return strings.TrimSpace(v)
		}
		return fallback
	}
	c := Config{MetricsEnabled: true, IPv6: true}
	c.Router.Address = value("IKUAI_ADDR", "http://192.168.1.1")
	c.Router.Token = value("IKUAI_TOKEN", "")
	if file := value("IKUAI_TOKEN_FILE", ""); file != "" {
		if c.Router.Token != "" {
			return c, errors.New("set only one of IKUAI_TOKEN and IKUAI_TOKEN_FILE")
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return c, fmt.Errorf("IKUAI_TOKEN_FILE: %w", err)
		}
		c.Router.Token = strings.TrimSpace(string(data))
	}
	var err error
	for _, item := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"HTTP_TIMEOUT", "10s", &c.Router.Timeout},
		{"IKUAI_SCRAPE_TIMEOUT", "15s", &c.ScrapeTimeout},
		{"IKUAI_JOB_TIMEOUT", "5m", &c.JobTimeout},
	} {
		*item.target, err = time.ParseDuration(value(item.key, item.fallback))
		if err != nil || *item.target <= 0 {
			return c, fmt.Errorf("%s must be a positive duration", item.key)
		}
	}
	var disabled bool
	for _, item := range []struct {
		key, fallback string
		target        *bool
	}{
		{"HTTP_INSECURE_SKIP_VERIFY", "false", &c.Router.InsecureSkipVerify},
		{"IKUAI_EXPORTER_DISABLE", "false", &disabled},
		{"IKUAI_COLLECT_IPV6", "true", &c.IPv6},
		{"IKUAI_CRON_SKIP_START", "false", &c.SkipStart},
	} {
		*item.target, err = strconv.ParseBool(value(item.key, item.fallback))
		if err != nil {
			return c, fmt.Errorf("%s must be a boolean", item.key)
		}
	}
	c.MetricsEnabled = !disabled
	c.Router.ReadRetries, err = strconv.Atoi(value("HTTP_READ_RETRIES", "2"))
	if err != nil {
		return c, errors.New("HTTP_READ_RETRIES must be an integer from 0 to 5")
	}
	c.Timezone, err = time.LoadLocation(value("TZ", "Asia/Shanghai"))
	if err != nil {
		return c, fmt.Errorf("TZ: %w", err)
	}
	c.ListenAddress = value("IKUAI_EXPORTER_LISTEN_ADDR", "0.0.0.0:8000")
	_, port, err := net.SplitHostPort(c.ListenAddress)
	if err != nil {
		return c, errors.New("IKUAI_EXPORTER_LISTEN_ADDR must be host:port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return c, errors.New("invalid exporter port")
	}
	client, err := api.NewClient(c.Router)
	if err != nil {
		return c, fmt.Errorf("router configuration: %w", err)
	}
	client.Close()

	pattern := regexp.MustCompile(`^IKUAI_(IP_OBJECT|DOMAIN_RULE)_(\d+)$`)
	type indexed struct {
		key, kind string
		index     int
	}
	var keys []indexed
	for key := range env {
		if key == "IKUAI_USERNAME" || key == "IKUAI_PASSWORD" || strings.HasPrefix(key, "IKUAI_CRON_CUSTOM_ISP_") || strings.HasPrefix(key, "IKUAI_CRON_STREAM_DOMAIN_") {
			return c, fmt.Errorf("%s is a removed 3.x setting; configure a 4.x token and JSON tasks", key)
		}
		if match := pattern.FindStringSubmatch(key); match != nil {
			index, e := strconv.Atoi(match[2])
			if e != nil || index < 1 {
				return c, fmt.Errorf("%s requires a positive task index", key)
			}
			keys = append(keys, indexed{key: key, kind: match[1], index: index})
		} else if strings.HasPrefix(key, "IKUAI_IP_OBJECT_") || strings.HasPrefix(key, "IKUAI_DOMAIN_RULE_") {
			return c, fmt.Errorf("%s requires a positive numeric task suffix", key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].index == keys[j].index {
			return keys[i].key < keys[j].key
		}
		return keys[i].index < keys[j].index
	})
	seen := map[string]bool{}
	for _, key := range keys {
		var task Task
		if key.kind == "IP_OBJECT" {
			entry := IPObjectTask{}
			if err = decodeTask(env[key.key], &entry); err != nil {
				return c, fmt.Errorf("%s: %w", key.key, err)
			}
			task = entry.Task
			if !validName(task.Name, 8, false) {
				return c, fmt.Errorf("%s: IP object prefix must contain 1-8 Chinese/ASCII letters or digits", key.key)
			}
			c.IPObjects = append(c.IPObjects, entry)
		} else {
			entry := DomainRuleTask{Priority: 31}
			if err = decodeTask(env[key.key], &entry); err != nil {
				return c, fmt.Errorf("%s: %w", key.key, err)
			}
			task = entry.Task
			if !validName(task.Name, 15, true) {
				return c, fmt.Errorf("%s: invalid domain rule name (1-15 characters)", key.key)
			}
			if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(entry.Interface) {
				return c, fmt.Errorf("%s: one interface is required", key.key)
			}
			if entry.Priority < 0 || entry.Priority > 63 {
				return c, fmt.Errorf("%s: priority must be 0-63", key.key)
			}
			for _, source := range entry.Source {
				if !validSource(source) {
					return c, fmt.Errorf("%s: invalid source address", key.key)
				}
			}
			c.DomainRules = append(c.DomainRules, entry)
		}
		if err = validateTask(task); err != nil {
			return c, fmt.Errorf("%s: %w", key.key, err)
		}
		identity := key.kind + "/" + task.Name
		if seen[identity] {
			return c, fmt.Errorf("%s: duplicate managed resource name", key.key)
		}
		seen[identity] = true
	}
	return c, nil
}

func decodeTask(raw string, output any) error {
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(output); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("task must contain exactly one JSON object")
	}
	return nil
}

func validateTask(task Task) error {
	if _, err := schedule.Parse(task.Schedule); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	if len(task.URLs) == 0 {
		return errors.New("urls must not be empty")
	}
	for _, source := range task.URLs {
		u, err := url.Parse(source)
		if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return errors.New("urls must be http(s) URLs without credentials or fragments")
		}
	}
	if utf8.RuneCountInString(task.Comment) > 64 {
		return errors.New("comment must not exceed 64 characters")
	}
	return nil
}

func validName(name string, maximum int, symbols bool) bool {
	if n := utf8.RuneCountInString(name); n < 1 || n > maximum {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fa5) {
			continue
		}
		if symbols && i > 0 && (r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}

func validSource(source string) bool {
	if _, err := netip.ParseAddr(source); err == nil {
		return true
	}
	if _, err := netip.ParsePrefix(source); err == nil {
		return true
	}
	if mac, err := net.ParseMAC(source); err == nil && len(mac) == 6 {
		return true
	}
	if start, end, ok := strings.Cut(source, "-"); ok {
		a, e1 := netip.ParseAddr(start)
		b, e2 := netip.ParseAddr(end)
		return e1 == nil && e2 == nil && a.Is4() == b.Is4() && a.Compare(b) <= 0
	}
	return false
}
