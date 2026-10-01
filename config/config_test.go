package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(values ...string) []string { return append([]string{"IKUAI_TOKEN=test-token"}, values...) }

func TestDeterministicTasksAndPriority(t *testing.T) {
	c, err := Read(env(
		`IKUAI_IP_OBJECT_10={"schedule":"8h","name":"China","urls":["https://example.com/cn"]}`,
		`IKUAI_IP_OBJECT_2={"schedule":"0 6 * * *","name":"Telegram","urls":["https://example.com/tg"]}`,
		`IKUAI_DOMAIN_RULE_1={"schedule":"10m","name":"GFW","interface":"wan2","urls":["https://example.com/gfw"],"source":["192.168.1.10-192.168.1.20"]}`,
		`IKUAI_DOMAIN_RULE_2={"schedule":"10m","name":"Other","interface":"wan1","urls":["https://example.com/other"],"priority":0}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	if c.IPObjects[0].Name != "Telegram" || c.IPObjects[1].Name != "China" {
		t.Fatal("task order is unstable")
	}
	if c.DomainRules[0].Priority != 31 || c.DomainRules[1].Priority != 0 {
		t.Fatal("priority default overwrote explicit zero")
	}
	if !c.MetricsEnabled || !c.IPv6 || c.Timezone.String() != "Asia/Shanghai" {
		t.Fatal("incorrect defaults")
	}
}

func TestInvalidConfigurationFailsEarly(t *testing.T) {
	for _, value := range []string{
		"IKUAI_TOKEN=", "IKUAI_TOKEN=bad\ntoken", "IKUAI_ADDR=ftp://router", "IKUAI_ADDR=http://user:pass@router",
		"HTTP_TIMEOUT=0s", "HTTP_READ_RETRIES=6", "TZ=not-a-timezone", "HTTP_INSECURE_SKIP_VERIFY=maybe", "IKUAI_EXPORTER_LISTEN_ADDR=localhost:99999",
		"IKUAI_USERNAME=admin", "IKUAI_CRON_CUSTOM_ISP_1=8h|China|https://example.com/list", "IKUAI_IP_OBJECT_bad={}",
		`IKUAI_IP_OBJECT_1={"schedule":"0s","name":"China","urls":["https://example.com/list"]}`,
		`IKUAI_IP_OBJECT_1={"schedule":"@every -1s","name":"China","urls":["https://example.com/list"]}`,
		`IKUAI_IP_OBJECT_1={"schedule":"8h","name":"TooLongName","urls":["https://example.com/list"]}`,
		`IKUAI_IP_OBJECT_1={"schedule":"8h","name":"China","urls":["file:///tmp/list"]}`,
		`IKUAI_IP_OBJECT_1={"schedule":"8h","name":"China","urls":["https://example.com/list"],"typo":true}`,
		`IKUAI_IP_OBJECT_1={"schedule":"8h","name":"China","urls":["https://example.com/list"]} {}`,
		`IKUAI_DOMAIN_RULE_1={"schedule":"10m","name":"GFW","interface":"wan1,wan2","urls":["https://example.com/list"]}`,
		`IKUAI_DOMAIN_RULE_1={"schedule":"10m","name":"GFW","interface":"wan2","urls":["https://example.com/list"],"priority":64}`,
		`IKUAI_DOMAIN_RULE_1={"schedule":"10m","name":"GFW","interface":"wan2","urls":["https://example.com/list"],"source":["192.168.1.20-192.168.1.10"]}`,
	} {
		t.Run(strings.SplitN(value, "=", 2)[0], func(t *testing.T) {
			if _, err := Read(env(value)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestDuplicateManagedNames(t *testing.T) {
	value := `{"schedule":"8h","name":"China","urls":["https://example.com/list"]}`
	if _, err := Read(env("IKUAI_IP_OBJECT_1="+value, "IKUAI_IP_OBJECT_2="+value)); err == nil {
		t.Fatal("duplicate ownership accepted")
	}
}

func TestTokenFileAndExclusiveSettings(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("file-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Read([]string{"IKUAI_TOKEN_FILE=" + file})
	if err != nil || c.Router.Token != "file-token" {
		t.Fatalf("token file failed: %v", err)
	}
	if _, err = Read(env("IKUAI_TOKEN_FILE=" + file)); err == nil {
		t.Fatal("ambiguous token sources accepted")
	}
}
