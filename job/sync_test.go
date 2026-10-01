package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/NERVEbing/ikuai-aio/v4/config"
)

type fakeSource struct {
	rows map[string][]string
	fail string
}

func (s fakeSource) Fetch(_ context.Context, url string) ([]string, error) {
	if url == s.fail {
		return nil, errors.New("download failed")
	}
	return s.rows[url], nil
}

type fakeRouter struct {
	objects []api.IPObject
	rules   []api.DomainRule
	events  []string
	created []api.IPObjectInput
	updated []api.IPObjectInput
	domain  api.DomainRuleInput
	fail    string
}

func (r *fakeRouter) IPObjects(context.Context) ([]api.IPObject, error) {
	r.events = append(r.events, "list-ip")
	return r.objects, nil
}
func (r *fakeRouter) CreateIPObject(_ context.Context, input api.IPObjectInput) error {
	r.events = append(r.events, "create:"+input.Name)
	r.created = append(r.created, input)
	if r.fail == "create" {
		return errors.New("write failed")
	}
	return nil
}
func (r *fakeRouter) UpdateIPObject(_ context.Context, id int64, input api.IPObjectInput) error {
	r.events = append(r.events, fmt.Sprintf("update:%d", id))
	r.updated = append(r.updated, input)
	if r.fail == "update" {
		return errors.New("write failed")
	}
	return nil
}
func (r *fakeRouter) DeleteIPObject(_ context.Context, id int64) error {
	r.events = append(r.events, fmt.Sprintf("delete:%d", id))
	return nil
}
func (r *fakeRouter) DomainRules(context.Context) ([]api.DomainRule, error) {
	r.events = append(r.events, "list-domain")
	return r.rules, nil
}
func (r *fakeRouter) CreateDomainRule(_ context.Context, input api.DomainRuleInput) error {
	r.events = append(r.events, "create-domain")
	r.domain = input
	return nil
}
func (r *fakeRouter) UpdateDomainRule(_ context.Context, id int64, input api.DomainRuleInput) error {
	r.events = append(r.events, fmt.Sprintf("update-domain:%d", id))
	r.domain = input
	return nil
}

func ipRows(count int) []string {
	var rows []string
	for i := 1; i <= count; i++ {
		rows = append(rows, fmt.Sprintf("10.0.%d.%d", i/255, i%255))
	}
	return rows
}

func TestIPObjectsPreserveIDsAndCleanUpLast(t *testing.T) {
	router := &fakeRouter{objects: []api.IPObject{
		{ID: 7, IPObjectInput: api.IPObjectInput{Name: "ChinaAIO0001", Values: []api.IPValue{{IP: "1.1.1.1"}}}},
		{ID: 9, IPObjectInput: api.IPObjectInput{Name: "ChinaAIO0003"}},
		{ID: 99, IPObjectInput: api.IPObjectInput{Name: "ManualObject"}},
	}}
	worker := NewWorker(router, fakeSource{rows: map[string][]string{"source": ipRows(101)}})
	task := config.IPObjectTask{Task: config.Task{Name: "China", URLs: []string{"source"}}}
	if err := worker.SyncIPObjects(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	want := []string{"list-ip", "create:ChinaAIO0002", "update:7", "delete:9"}
	if !reflect.DeepEqual(router.events, want) {
		t.Fatalf("unsafe reconciliation order: %v", router.events)
	}
	if len(router.created[0].Values) != 1 || len(router.updated[0].Values) != 100 {
		t.Fatal("object size limit violated")
	}
}

func TestSourceFailuresNeverTouchRouter(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		fail string
	}{
		{"unavailable", []string{"1.1.1.1"}, "second"},
		{"invalid entry", []string{"1.1.1.1", "not-an-ip"}, ""},
		{"empty source", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := &fakeRouter{}
			worker := NewWorker(router, fakeSource{rows: map[string][]string{"first": {"2.2.2.2"}, "second": tc.rows}, fail: tc.fail})
			err := worker.SyncIPObjects(context.Background(), config.IPObjectTask{Task: config.Task{Name: "China", URLs: []string{"first", "second"}}})
			if err == nil || len(router.events) != 0 {
				t.Fatalf("router touched before all sources validated: %v %v", err, router.events)
			}
		})
	}
}

func TestFailedWritesDoNotDeleteExistingObjects(t *testing.T) {
	for _, failure := range []string{"create", "update"} {
		t.Run(failure, func(t *testing.T) {
			router := &fakeRouter{fail: failure, objects: []api.IPObject{{ID: 1, IPObjectInput: api.IPObjectInput{Name: "ChinaAIO0001"}}, {ID: 3, IPObjectInput: api.IPObjectInput{Name: "ChinaAIO0003"}}}}
			worker := NewWorker(router, fakeSource{rows: map[string][]string{"source": ipRows(101)}})
			if err := worker.SyncIPObjects(context.Background(), config.IPObjectTask{Task: config.Task{Name: "China", URLs: []string{"source"}}}); err == nil {
				t.Fatal("failed write accepted")
			}
			for _, event := range router.events {
				if strings.HasPrefix(event, "delete:") {
					t.Fatal("old objects deleted after failed write")
				}
			}
		})
	}
}

func TestIdenticalIPObjectIsNotRewritten(t *testing.T) {
	input := api.IPObjectInput{Name: "ChinaAIO0001", Values: []api.IPValue{{IP: "1.1.1.1"}}}
	router := &fakeRouter{objects: []api.IPObject{{ID: 7, IPObjectInput: input}}}
	worker := NewWorker(router, fakeSource{rows: map[string][]string{"source": {"1.1.1.1", "1.1.1.1/32"}}})
	if err := worker.SyncIPObjects(context.Background(), config.IPObjectTask{Task: config.Task{Name: "China", URLs: []string{"source"}}}); err != nil {
		t.Fatal(err)
	}
	if len(router.events) != 1 {
		t.Fatalf("unnecessary mutations: %v", router.events)
	}
}

func TestDomainUpsertUsesNestedV4FieldsAndExistingID(t *testing.T) {
	router := &fakeRouter{rules: []api.DomainRule{{ID: 42, Name: "GFW"}, {ID: 9, Name: "Other"}}}
	worker := NewWorker(router, fakeSource{rows: map[string][]string{"source": {"Example.COM.", "example.com", "test.example"}}})
	task := config.DomainRuleTask{Task: config.Task{Name: "GFW", URLs: []string{"source"}}, Interface: "wan2", Priority: 31, Source: []string{"192.168.1.0/24"}}
	if err := worker.SyncDomainRule(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(router.events, []string{"list-domain", "update-domain:42"}) {
		t.Fatalf("rule ID was replaced: %v", router.events)
	}
	if !reflect.DeepEqual(router.domain.Domain.Custom, []string{"example.com", "test.example"}) || router.domain.Source.Custom[0] != "192.168.1.0/24" || router.domain.Time.Custom[0].Weekdays != "1234567" || router.domain.Enabled != "yes" {
		t.Fatalf("incorrect v4 rule: %+v", router.domain)
	}
}

func TestAmbiguousResourceNamesAbortWrites(t *testing.T) {
	router := &fakeRouter{rules: []api.DomainRule{{ID: 1, Name: "GFW"}, {ID: 2, Name: "GFW"}}}
	worker := NewWorker(router, fakeSource{rows: map[string][]string{"source": {"example.com"}}})
	if err := worker.SyncDomainRule(context.Background(), config.DomainRuleTask{Task: config.Task{Name: "GFW", URLs: []string{"source"}}}); err == nil || len(router.events) != 1 {
		t.Fatal("ambiguous name was mutated")
	}
}

func TestListNormalization(t *testing.T) {
	for input, want := range map[string]string{"192.168.1.8/24": "192.168.1.0/24", "1.1.1.1/32": "1.1.1.1", "1.1.1.1 - 1.1.1.3": "1.1.1.1-1.1.1.3"} {
		got, err := normalizeIPv4(input)
		if err != nil || got != want {
			t.Fatalf("%q => %q %v", input, got, err)
		}
	}
	for _, input := range []string{"256.1.1.1", "::1", "1.1.1.3-1.1.1.1", "1.1.1.1/33"} {
		if _, err := normalizeIPv4(input); err == nil {
			t.Fatalf("invalid IP accepted: %s", input)
		}
	}
	for _, input := range []string{"example..com", "-bad.com", "bad-.com", "https://example.com", "bad_thing.com", "*.example.com"} {
		if _, err := normalizeDomain(input); err == nil {
			t.Fatalf("invalid hostname accepted: %s", input)
		}
	}
}

type blockingSource struct {
	started           chan struct{}
	active, maxActive atomic.Int32
}

func (s *blockingSource) Fetch(ctx context.Context, _ string) ([]string, error) {
	n := s.active.Add(1)
	defer s.active.Add(-1)
	for old := s.maxActive.Load(); n > old; old = s.maxActive.Load() {
		if s.maxActive.CompareAndSwap(old, n) {
			break
		}
	}
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSchedulerCancelsActiveAndQueuedTasks(t *testing.T) {
	source := &blockingSource{started: make(chan struct{}, 2)}
	worker := NewWorker(&fakeRouter{}, source)
	conf := config.Config{Timezone: time.UTC, JobTimeout: time.Hour, IPObjects: []config.IPObjectTask{{Task: config.Task{Name: "One", Schedule: "1s", URLs: []string{"source"}}}, {Task: config.Task{Name: "Two", Schedule: "1s", URLs: []string{"source"}}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, conf, worker, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("startup task did not run")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
	if source.maxActive.Load() != 1 || source.active.Load() != 0 {
		t.Fatal("tasks overlapped or remained active")
	}
}
