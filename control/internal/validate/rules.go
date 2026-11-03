package validate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"corvus/control/internal/netcfg"
	"corvus/control/internal/store"
)

// Limits for security rules.
const (
	MaxRules         = 200
	MaxGroups        = 10
	MaxConditions    = 10
	MaxValueLen      = 1024
	MaxValues        = 1000
	MaxRegexLen      = 512
	MaxIPListItems   = 50000
	maxRuleNameRunes = 100
)

var stringOps = map[string]bool{
	"eq": true, "ne": true, "contains": true, "not_contains": true, "starts_with": true, "ends_with": true,
	"matches": true, "not_matches": true, "in": true, "not_in": true,
}

var setOps = map[string]bool{"in": true, "not_in": true}

// RuleFields lists the fields a condition may test and whether exists/not_exists apply.
var RuleFields = map[string]struct{ set, str, exists bool }{
	"ip.src":          {set: true},
	"ip.country":      {set: true},
	"http.method":     {set: true},
	"http.scheme":     {set: true},
	"http.host":       {str: true},
	"http.path":       {str: true},
	"http.query":      {str: true},
	"http.user_agent": {str: true, exists: true},
	"http.referer":    {str: true, exists: true},
	"http.header":     {str: true, exists: true},
	"http.body":       {str: true},
}

// bodyOps: request bodies are matched as normalised text (see gateway corvus/body.lua).
var bodyOps = map[string]bool{"contains": true, "not_contains": true, "matches": true, "not_matches": true}

var (
	headerNameRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	countryRe    = regexp.MustCompile(`^[A-Z]{2}$`)
	methodRe     = regexp.MustCompile(`^[A-Z]{1,16}$`)
)

// DisplayName validates a free-form display name (any language, no control characters).
func DisplayName(s string, max int) bool {
	if s == "" || utf8.RuneCountInString(s) > max || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// SecurityRule validates and normalises a rule. listExists reports whether an IP list id exists;
// hostExists whether a proxy host id exists.
func SecurityRule(r *store.SecurityRule, listExists, hostExists func(int64) bool) error {
	var errs Errors
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if !DisplayName(r.Name, maxRuleNameRunes) {
		errs.add("name", "invalid", "name must be 1-100 characters")
	}
	if utf8.RuneCountInString(r.Description) > 500 || strings.ContainsAny(r.Description, "\x00") {
		errs.add("description", "invalid", "description must be at most 500 characters")
	}
	switch r.Action {
	case "block":
		switch r.StatusCode {
		case 0:
			r.StatusCode = 403
		case 401, 403, 404, 410, 429, 444:
		default:
			errs.add("status_code", "invalid", "status code must be 401, 403, 404, 410, 429 or 444")
		}
		r.SkipBasicAuth = false
	case "allow":
		r.StatusCode = 403
	case "log":
		r.StatusCode, r.SkipBasicAuth = 403, false
	default:
		errs.add("action", "invalid", "action must be block, allow or log")
	}
	switch r.Scope {
	case "", "all":
		r.Scope, r.HostIDs = "all", []int64{}
	case "hosts":
		if len(r.HostIDs) == 0 {
			errs.add("host_ids", "required", "select at least one proxy host")
		}
		seen := map[int64]bool{}
		ids := []int64{}
		for _, h := range r.HostIDs {
			if hostExists != nil && !hostExists(h) {
				errs.add("host_ids", "not_found", "proxy host %d does not exist", h)
			} else if !seen[h] {
				seen[h] = true
				ids = append(ids, h)
			}
		}
		r.HostIDs = ids
	default:
		errs.add("scope", "invalid", "scope must be all or hosts")
	}
	groups := r.Expression.Groups
	if len(groups) == 0 {
		errs.add("expression", "required", "add at least one condition")
	}
	if len(groups) > MaxGroups {
		errs.add("expression", "too_many", "at most %d OR groups", MaxGroups)
	}
	for gi := range groups {
		if len(groups[gi]) == 0 {
			errs.add("expression", "invalid", "empty condition group")
		}
		if len(groups[gi]) > MaxConditions {
			errs.add("expression", "too_many", "at most %d conditions per group", MaxConditions)
		}
		for ci := range groups[gi] {
			if err := condition(&groups[gi][ci], listExists); err != nil {
				errs.add(fmt.Sprintf("expression.groups.%d.%d", gi, ci), "invalid", "%s", err.Error())
			}
		}
	}
	return errs.OrNil()
}

func condition(c *store.Condition, listExists func(int64) bool) error {
	f, ok := RuleFields[c.Field]
	if !ok {
		return fmt.Errorf("unknown field %q", c.Field)
	}
	if c.Field == "http.header" {
		c.Name = strings.ToLower(strings.TrimSpace(c.Name))
		if !headerNameRe.MatchString(c.Name) {
			return fmt.Errorf("a valid header name is required")
		}
	} else {
		c.Name = ""
	}
	switch {
	case (c.Operator == "exists" || c.Operator == "not_exists") && f.exists:
		c.Value = []string{}
		return nil
	case f.set && !setOps[c.Operator], f.str && !stringOps[c.Operator], c.Field == "http.body" && !bodyOps[c.Operator]:
		return fmt.Errorf("operator %q is not valid for %s", c.Operator, c.Field)
	}
	vals := make([]string, 0, len(c.Value))
	for _, v := range c.Value {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if len(v) > MaxValueLen || strings.ContainsAny(v, "\x00\r\n") {
			return fmt.Errorf("values must be single-line and at most %d bytes", MaxValueLen)
		}
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		return fmt.Errorf("a value is required")
	}
	if len(vals) > MaxValues {
		return fmt.Errorf("at most %d values", MaxValues)
	}
	if !setOps[c.Operator] && len(vals) != 1 {
		return fmt.Errorf("operator %q takes exactly one value", c.Operator)
	}
	switch c.Field {
	case "ip.src":
		var cidrs []string
		for _, v := range vals {
			if id, ok := store.ListRef(v); ok {
				if listExists != nil && !listExists(id) {
					return fmt.Errorf("IP list %d does not exist", id)
				}
				continue
			}
			cidrs = append(cidrs, v)
		}
		if _, err := netcfg.NormalizeCIDRs(cidrs); err != nil {
			return err
		}
	case "ip.country":
		for i, v := range vals {
			vals[i] = strings.ToUpper(v)
			if !countryRe.MatchString(vals[i]) {
				return fmt.Errorf("invalid country code %q", v)
			}
		}
	case "http.method":
		for i, v := range vals {
			vals[i] = strings.ToUpper(v)
			if !methodRe.MatchString(vals[i]) {
				return fmt.Errorf("invalid method %q", v)
			}
		}
	case "http.scheme":
		for i, v := range vals {
			vals[i] = strings.ToLower(v)
			if vals[i] != "http" && vals[i] != "https" {
				return fmt.Errorf("scheme must be http or https")
			}
		}
	}
	if c.Operator == "matches" || c.Operator == "not_matches" {
		if err := RuleRegex(vals[0]); err != nil {
			return err
		}
	}
	c.Value = vals
	return nil
}

// RuleRegex accepts patterns that are valid RE2 (no backreferences or lookaround), which
// PCRE also understands. Backtracking at runtime is additionally capped by
// lua_regex_match_limit in the gateway.
func RuleRegex(p string) error {
	if len(p) > MaxRegexLen {
		return fmt.Errorf("regular expression must be at most %d characters", MaxRegexLen)
	}
	// \C (single byte) and (*VERB) control verbs are PCRE-only; RE2 rejects the rest.
	if strings.Contains(p, `\C`) || strings.Contains(p, "(*") {
		return fmt.Errorf("unsupported regular expression construct")
	}
	if _, err := regexp.Compile(p); err != nil {
		return fmt.Errorf("invalid regular expression: %v", err)
	}
	return nil
}

// IPList validates and normalises an IP list.
func IPList(l *store.IPList) error {
	var errs Errors
	l.Name = strings.TrimSpace(l.Name)
	l.Description = strings.TrimSpace(l.Description)
	if !DisplayName(l.Name, 64) || strings.ContainsAny(l.Name, `$"\`) {
		errs.add("name", "invalid", "name must be 1-64 characters (no $ \" \\)")
	}
	if utf8.RuneCountInString(l.Description) > 500 {
		errs.add("description", "invalid", "description must be at most 500 characters")
	}
	if len(l.Items) > MaxIPListItems {
		errs.add("items", "too_many", "at most %d entries", MaxIPListItems)
	} else if items, err := netcfg.NormalizeCIDRs(l.Items); err != nil {
		errs.add("items", "invalid", "%s", err.Error())
	} else {
		l.Items = items
	}
	return errs.OrNil()
}
