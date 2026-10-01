package exporter

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/api"
)

func TestHTTPExporterAgainstV4Router(t *testing.T) {
	var lock sync.Mutex
	requests := map[string]int{}
	router := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer integration-token" {
			t.Errorf("unexpected authentication: %s", r.Method)
		}
		lock.Lock()
		requests[r.URL.Path]++
		lock.Unlock()
		switch r.URL.Path {
		case api.BasePath + "/monitoring/system":
			fmt.Fprint(w, `{"message":"Success","results":{"sysinfo":{"verinfo":{"version":"4.0.222","arch":"x86"},"cpu":["10%"],"memory":{"total":1000,"available":600,"cached":0,"buffers":0},"stream":{"upload":0,"download":0,"total_up":0,"total_down":0,"connect_num":0},"uptime":120,"online_user":{"count":1}}}}`)
		case api.BasePath + "/monitoring/interfaces-status":
			fmt.Fprint(w, `{"message":"Success","results":{"iface_check":nil,"iface_stream":[]}}`)
		case api.BasePath + "/monitoring/clients-online":
			fmt.Fprint(w, `{"message":"Success","results":{"total":1,"data":[{"id":1,"ip_addr":"192.168.1.2","termname":"Laptop","total_up":12}]}}`)
		case api.BasePath + "/monitoring/clients-ip6-online":
			fmt.Fprint(w, `{"message":"Success","results":{"total":0,"data":nil}}`)
		default:
			t.Errorf("non-v4 path used: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer router.Close()
	client, err := api.NewClient(api.Options{Address: router.URL, Token: "integration-token", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	metrics := NewMetrics(context.Background(), client, time.Second, true, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler, err := NewHandler(metrics, true)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 {
		t.Fatalf("scrape HTTP %d: %s", response.Code, response.Body.String())
	}
	for _, value := range []string{"ikuai_scrape_success 1", "ikuai_memory_used_bytes 409600", `ikuai_up{id="host"} 1`, `name="Laptop"`, `version="4.0.222"`} {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("missing metric %q", value)
		}
	}
	lock.Lock()
	defer lock.Unlock()
	if len(requests) != 4 {
		t.Fatalf("expected four REST endpoints, got %v", requests)
	}
}
