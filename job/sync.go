package job

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/NERVEbing/ikuai-aio/v4/api"
	"github.com/NERVEbing/ikuai-aio/v4/config"
)

type Router interface {
	IPObjects(context.Context) ([]api.IPObject, error)
	CreateIPObject(context.Context, api.IPObjectInput) error
	UpdateIPObject(context.Context, int64, api.IPObjectInput) error
	DeleteIPObject(context.Context, int64) error
	DomainRules(context.Context) ([]api.DomainRule, error)
	CreateDomainRule(context.Context, api.DomainRuleInput) error
	UpdateDomainRule(context.Context, int64, api.DomainRuleInput) error
}

type Source interface {
	Fetch(context.Context, string) ([]string, error)
}

type Worker struct {
	router Router
	source Source
}

func NewWorker(router Router, source Source) *Worker { return &Worker{router: router, source: source} }

func (w *Worker) download(ctx context.Context, urls []string, normalize func(string) (string, error)) ([]string, error) {
	var all []string
	for index, source := range urls {
		rows, err := w.source.Fetch(ctx, source)
		if err != nil {
			return nil, fmt.Errorf("source %d: %w", index+1, err)
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("source %d is empty", index+1)
		}
		for line, row := range rows {
			entry, err := normalize(row)
			if err != nil {
				return nil, fmt.Errorf("source %d entry %d: %w", index+1, line+1, err)
			}
			all = append(all, entry)
		}
	}
	if len(all) == 0 {
		return nil, errors.New("refusing to replace a list with no entries")
	}
	return uniqueSorted(all), nil
}

// SyncIPObjects stores a large list in stable, named objects of at most 100
// entries. New objects are created before existing IDs are updated, and stale
// objects are removed only after every desired write succeeded.
func (w *Worker) SyncIPObjects(ctx context.Context, task config.IPObjectTask) error {
	rows, err := w.download(ctx, task.URLs, normalizeIPv4)
	if err != nil {
		return err
	}
	if len(rows) > 999900 {
		return errors.New("IP list exceeds the managed object namespace")
	}
	existing, err := w.router.IPObjects(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]api.IPObject)
	for _, object := range existing {
		if !managedObject(object.Name, task.Name) {
			continue
		}
		if _, duplicate := byName[object.Name]; duplicate || object.ID < 1 {
			return errors.New("ambiguous managed IP object list")
		}
		byName[object.Name] = object
	}
	var desired []api.IPObjectInput
	for start := 0; start < len(rows); start += 100 {
		input := api.IPObjectInput{Name: fmt.Sprintf("%sAIO%04d", task.Name, len(desired)+1)}
		for _, row := range rows[start:min(start+100, len(rows))] {
			input.Values = append(input.Values, api.IPValue{IP: row, Comment: task.Comment})
		}
		desired = append(desired, input)
	}
	for _, input := range desired {
		if _, exists := byName[input.Name]; !exists {
			if err = w.router.CreateIPObject(ctx, input); err != nil {
				return fmt.Errorf("create %s: %w", input.Name, err)
			}
		}
	}
	for _, input := range desired {
		if old, exists := byName[input.Name]; exists && !reflect.DeepEqual(old.IPObjectInput, input) {
			if err = w.router.UpdateIPObject(ctx, old.ID, input); err != nil {
				return fmt.Errorf("update %s: %w", input.Name, err)
			}
		}
		delete(byName, input.Name)
	}
	var stale []api.IPObject
	for _, old := range byName {
		stale = append(stale, old)
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].Name < stale[j].Name })
	for _, old := range stale {
		if err = w.router.DeleteIPObject(ctx, old.ID); err != nil {
			return fmt.Errorf("remove stale %s (it may still be referenced): %w", old.Name, err)
		}
	}
	return nil
}

func managedObject(name, prefix string) bool {
	suffix, ok := strings.CutPrefix(name, prefix+"AIO")
	if !ok || len(suffix) != 4 {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	index, err := strconv.Atoi(suffix)
	return err == nil && index > 0
}

func (w *Worker) SyncDomainRule(ctx context.Context, task config.DomainRuleTask) error {
	rows, err := w.download(ctx, task.URLs, normalizeDomain)
	if err != nil {
		return err
	}
	rules, err := w.router.DomainRules(ctx)
	if err != nil {
		return err
	}
	var id int64
	for _, rule := range rules {
		if rule.Name != task.Name {
			continue
		}
		if id != 0 || rule.ID < 1 {
			return errors.New("ambiguous managed domain rule name")
		}
		id = rule.ID
	}
	sources := append([]string{}, task.Source...)
	input := api.DomainRuleInput{Name: task.Name, Interface: task.Interface, Enabled: "yes", Priority: task.Priority, Comment: task.Comment,
		Domain: api.CustomValues{Custom: rows}, Source: api.CustomValues{Custom: sources}}
	input.Time.Custom = []api.WeeklyTime{{Type: "weekly", Weekdays: "1234567", Start: "00:00", End: "23:59"}}
	if id != 0 {
		return w.router.UpdateDomainRule(ctx, id, input)
	}
	return w.router.CreateDomainRule(ctx, input)
}
