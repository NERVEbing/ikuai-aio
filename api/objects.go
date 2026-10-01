package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

func (c *Client) IPObjects(ctx context.Context) ([]IPObject, error) {
	return list[IPObject](ctx, c, "/ip-objects", "ip_data", "ip_total")
}

func (c *Client) CreateIPObject(ctx context.Context, input IPObjectInput) error {
	if len(input.Values) == 0 || len(input.Values) > 100 {
		return errors.New("an IP object must contain 1 to 100 entries")
	}
	_, err := c.request(ctx, http.MethodPost, "/ip-objects", nil, input)
	return err
}

func (c *Client) UpdateIPObject(ctx context.Context, id int64, input IPObjectInput) error {
	if id < 1 || len(input.Values) == 0 || len(input.Values) > 100 {
		return errors.New("invalid IP object ID or entry count")
	}
	_, err := c.request(ctx, http.MethodPut, fmt.Sprintf("/ip-objects/%d", id), nil, input)
	return err
}

func (c *Client) DeleteIPObject(ctx context.Context, id int64) error {
	if id < 1 {
		return errors.New("invalid IP object ID")
	}
	_, err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/ip-objects/%d", id), nil, nil)
	return err
}

func (c *Client) DomainRules(ctx context.Context) ([]DomainRule, error) {
	return list[DomainRule](ctx, c, "/routing/domain-rules", "data", "total")
}

func (c *Client) CreateDomainRule(ctx context.Context, input DomainRuleInput) error {
	_, err := c.request(ctx, http.MethodPost, "/routing/domain-rules", nil, input)
	return err
}

func (c *Client) UpdateDomainRule(ctx context.Context, id int64, input DomainRuleInput) error {
	if id < 1 {
		return errors.New("invalid domain rule ID")
	}
	_, err := c.request(ctx, http.MethodPut, fmt.Sprintf("/routing/domain-rules/%d", id), nil, input)
	return err
}

func (c *Client) DeleteDomainRule(ctx context.Context, id int64) error {
	if id < 1 {
		return errors.New("invalid domain rule ID")
	}
	_, err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/routing/domain-rules/%d", id), nil, nil)
	return err
}
