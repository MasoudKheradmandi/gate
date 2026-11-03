package validate

import (
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"corvus/control/internal/store"
)

// IP list names and entries end up in the generated gateway configuration, so whatever
// the validator accepts must be well-formed and safe to render.
func FuzzIPList(f *testing.F) {
	f.Add("office", "10.0.0.0/8\n203.0.113.7")
	f.Add("v6", "2001:db8::/32")
	f.Add("bad\"name", "1.2.3.4/33")
	f.Fuzz(func(t *testing.T, name, items string) {
		l := &store.IPList{Name: name, Items: strings.Split(items, "\n")}
		if IPList(l) != nil {
			return
		}
		if strings.ContainsAny(l.Name, `$"\`) {
			t.Fatalf("accepted unsafe name %q", l.Name)
		}
		for _, it := range l.Items {
			if _, err := netip.ParsePrefix(it); err != nil {
				t.Fatalf("accepted item %q is not a prefix: %v", it, err)
			}
		}
		again := &store.IPList{Name: l.Name, Items: append([]string(nil), l.Items...)}
		if err := IPList(again); err != nil {
			t.Fatalf("normalised list no longer validates: %v", err)
		}
	})
}

// Accepted rule patterns must compile and stay within the length limit.
func FuzzRuleRegex(f *testing.F) {
	f.Add(`^/wp-admin/`)
	f.Add(`(?i)union\s+select`)
	f.Add(`(a+)+$`)
	f.Fuzz(func(t *testing.T, p string) {
		if RuleRegex(p) != nil {
			return
		}
		if len(p) > MaxRegexLen {
			t.Fatalf("accepted pattern longer than %d", MaxRegexLen)
		}
		if _, err := regexp.Compile(p); err != nil {
			t.Fatalf("accepted pattern does not compile: %v", err)
		}
	})
}
