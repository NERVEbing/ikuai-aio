package job

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFetchPlainLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("router credentials leaked to source")
		}
		fmt.Fprint(w, "\ufeff# header\r\n; comment\n// comment\n\nfirst # annotation\n second \n")
	}))
	defer server.Close()
	rows, err := NewFetcher(time.Second).Fetch(context.Background(), server.URL)
	if err != nil || !reflect.DeepEqual(rows, []string{"first", "second"}) {
		t.Fatalf("list parse failed: %v %v", rows, err)
	}
}

func TestSourceRedirectsStayUnauthenticated(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials leaked")
		}
		fmt.Fprint(w, "example.com\n")
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	rows, err := NewFetcher(time.Second).Fetch(context.Background(), source.URL)
	if err != nil || len(rows) != 1 {
		t.Fatalf("legitimate source redirect failed: %v", err)
	}
}

func TestFetchErrorsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"status", "failed", 500}, {"empty", "# comment\n", 200}, {"oversize", strings.Repeat("#\r\n", maxSourceBytes/3+1), 200}, {"long line", strings.Repeat("x", 1<<20), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			if rows, err := NewFetcher(time.Second).Fetch(context.Background(), server.URL); err == nil {
				t.Fatalf("invalid source accepted (%d rows)", len(rows))
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := NewFetcher(time.Second).Fetch(ctx, server.URL); err == nil {
		t.Fatal("cancellation ignored")
	}
}
