package job

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

func normalizeIPv4(row string) (string, error) {
	row = strings.TrimSpace(row)
	if address, err := netip.ParseAddr(row); err == nil && address.Is4() {
		return address.String(), nil
	}
	if prefix, err := netip.ParsePrefix(row); err == nil && prefix.Addr().Is4() {
		if prefix.Bits() == 32 {
			return prefix.Addr().String(), nil
		}
		return prefix.Masked().String(), nil
	}
	if start, end, ok := strings.Cut(row, "-"); ok {
		a, e1 := netip.ParseAddr(strings.TrimSpace(start))
		b, e2 := netip.ParseAddr(strings.TrimSpace(end))
		if e1 == nil && e2 == nil && a.Is4() && b.Is4() && a.Compare(b) <= 0 {
			return a.String() + "-" + b.String(), nil
		}
	}
	return "", fmt.Errorf("invalid IPv4 address, prefix or range")
}

func normalizeDomain(row string) (string, error) {
	domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(row), "."))
	if len(domain) == 0 || len(domain) > 253 {
		return "", fmt.Errorf("invalid domain length")
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid domain label")
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return "", fmt.Errorf("domains must use ASCII or punycode hostname syntax")
			}
		}
	}
	return domain, nil
}

func uniqueSorted(rows []string) []string {
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		seen[row] = true
	}
	output := make([]string, 0, len(seen))
	for row := range seen {
		output = append(output, row)
	}
	sort.Strings(output)
	return output
}
