package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const systemJSON = `{"message":"Success","results":{"sysinfo":{"verinfo":{"version":"4.0.222","arch":"x86","verstring":"4.0.222 x64"},"cpu":["12%","15%"],"cputemp":[42.6],"memory":{"total":1024,"available":"512","cached":0,"buffers":0},"uptime":123,"stream":{"upload":10,"download":20,"total_up":100,"total_down":200,"connect_num":3},"online_user":{"count":2}}}}`

func testClient(t *testing.T, handler http.HandlerFunc, retries int) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Options{Address: server.URL, Token: "test-token", Timeout: time.Second, ReadRetries: retries})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestV4SystemAuthenticationAndNumbers(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != BasePath+"/monitoring/system" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Cookie") != "" {
			t.Error("expected Bearer authentication without cookies")
		}
		fmt.Fprint(w, systemJSON)
	}, 0)
	system, err := client.System(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if system.Memory.Available != 512 || system.CPUTemp[0] != 42.6 || system.Stream.TotalUp != 100 {
		t.Fatalf("unexpected system: %+v", system)
	}
}

func TestProtocolFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"unauthorized", `{"code":3007,"message":"invalid test-token"}`, 401},
		{"forbidden", `{"message":"Forbidden"}`, 403},
		{"business error", `{"code":30001,"message":"invalid","details":[{"field":"ip","msg":"bad"}]}`, 200},
		{"invalid JSON", `<html>login</html>`, 200},
		{"legacy envelope", `{"Result":30000,"ErrMsg":"Success","Data":{}}`, 200},
		{"missing results", `{"message":"Success"}`, 200},
		{"incomplete system", `{"message":"Success","results":{"sysinfo":{"verinfo":{"version":"4.0.222"}}}}`, 200},
		{"missing sysinfo", `{"message":"Success","results":{}}`, 200},
		{"unsupported firmware", strings.Replace(systemJSON, "4.0.222", "3.7.4", -1), 200},
		{"non success message", `{"message":"Denied","results":{}}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }, 0)
			_, err := client.System(context.Background())
			if err == nil {
				t.Fatal("invalid protocol response accepted")
			}
			if strings.Contains(err.Error(), "test-token") {
				t.Fatal("token leaked into error")
			}
			if tc.name == "business error" {
				var apiErr *Error
				if !errors.As(err, &apiErr) || apiErr.Code != 30001 || !strings.Contains(err.Error(), "ip: bad") {
					t.Fatalf("business error lost: %v", err)
				}
			}
		})
	}
}

func TestReadRetriesAndWritesNotReplayed(t *testing.T) {
	var gets, writes atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if gets.Add(1) == 1 {
				w.WriteHeader(503)
				fmt.Fprint(w, `{"message":"unavailable"}`)
				return
			}
			fmt.Fprint(w, systemJSON)
		} else {
			writes.Add(1)
			w.WriteHeader(503)
			fmt.Fprint(w, `{"message":"unavailable"}`)
		}
	}, 2)
	if _, err := client.System(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.CreateIPObject(context.Background(), IPObjectInput{Name: "Test", Values: []IPValue{{IP: "1.2.3.4"}}}); err == nil {
		t.Fatal("failed write accepted")
	}
	if gets.Load() != 2 || writes.Load() != 1 {
		t.Fatalf("gets=%d writes=%d", gets.Load(), writes.Load())
	}
}

func TestDeadlineInterruptsRetryAfter(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Retry-After", "30"); w.WriteHeader(429) }, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.System(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("deadline not respected: %v", err)
	}
}

func TestRedirectDoesNotForwardToken(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1); fmt.Fprint(w, systemJSON) }))
	defer target.Close()
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}, 2)
	if _, err := client.System(context.Background()); err == nil {
		t.Fatal("redirect accepted")
	}
	if forwarded.Load() != 0 {
		t.Fatal("credentials followed redirect")
	}
}

func TestNilNormalizationPreservesStrings(t *testing.T) {
	raw := []byte(`{"message":"Success","results":{"data":nil,"name":"vanilla nil","quote":"\\\"nil"},"values":[nil,"nil"]}`)
	var value map[string]any
	if err := json.Unmarshal(normalizeNil(raw), &value); err != nil {
		t.Fatal(err)
	}
	results := value["results"].(map[string]any)
	if results["name"] != "vanilla nil" || results["data"] != nil {
		t.Fatalf("strings or values corrupted: %v", results)
	}
}

func TestPaginationUsesEndpointSpecificKeysAndServerPageCap(t *testing.T) {
	var pages []int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pages = append(pages, page)
		if r.URL.Query().Get("limit") != "500" || r.URL.Query().Get("order_by") != "id" {
			t.Error("pagination parameters absent")
		}
		if page == 1 {
			fmt.Fprint(w, `{"message":"Success","results":{"ip_total":3,"ip_data":[{"id":1,"group_name":"a"},{"id":2,"group_name":"b"}]}}`)
		} else {
			fmt.Fprint(w, `{"message":"Success","results":{"ip_total":3,"ip_data":[{"id":3,"group_name":"c"}]}}`)
		}
	}, 0)
	objects, err := client.IPObjects(context.Background())
	if err != nil || len(objects) != 3 || len(pages) != 2 || pages[1] != 2 {
		t.Fatalf("incomplete pagination: %v %+v pages=%v", err, objects, pages)
	}
}

func TestPaginationFailureAndEmptyList(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"nil empty", `{"message":"Success","results":{"total":0,"data":nil}}`, false},
		{"incomplete", `{"message":"Success","results":{"total":5,"data":[]}}`, true},
		{"missing total", `{"message":"Success","results":{"data":[]}}`, true},
		{"null total", `{"message":"Success","results":{"total":null,"data":[]}}`, true},
		{"repeated page", `{"message":"Success","results":{"total":5,"data":[{"id":1}]}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, tc.body) }, 0)
			_, err := client.OnlineClients(context.Background(), true)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}

func TestV4WritePayloads(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PUT" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected write %s", r.Method)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, exists := body["func_name"]; exists {
			t.Error("legacy protocol used")
		}
		switch r.URL.Path {
		case BasePath + "/ip-objects/7":
			if string(body["group_name"]) != `"ChinaAIO0001"` || body["group_value"] == nil {
				t.Error("invalid object body")
			}
		case BasePath + "/routing/domain-rules/8":
			if string(body["domain"]) != `{"custom":["example.com"]}` || body["src_addr"] == nil || body["time"] == nil {
				t.Error("invalid nested rule body")
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":0,"message":"Success"}`)
	}, 0)
	if err := client.UpdateIPObject(context.Background(), 7, IPObjectInput{Name: "ChinaAIO0001", Values: []IPValue{{IP: "1.2.3.0/24"}}}); err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateDomainRule(context.Background(), 8, DomainRuleInput{Name: "GFW", Domain: CustomValues{Custom: []string{"example.com"}}}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("missing writes")
	}
}
