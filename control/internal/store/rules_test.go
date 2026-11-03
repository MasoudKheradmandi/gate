package store

import (
	"context"
	"testing"
)

func TestAccessListRulesSemantics(t *testing.T) {
	// satisfy any + users: allowed IPs skip the password; no block rule (password still works)
	r := AccessListRules("office", "any", []string{"10.0.0.0/8"}, []string{"all"}, true)
	if len(r) != 1 || r[0].Action != "allow" || !r[0].SkipBasicAuth {
		t.Fatalf("any+users: %+v", r)
	}
	// satisfy all + users: IP must match AND password → allow (no skip) + block rest
	r = AccessListRules("office", "all", []string{"10.0.0.0/8"}, []string{"all"}, true)
	if len(r) != 2 || r[0].SkipBasicAuth || r[1].Action != "block" || r[1].Expression.Groups[0][0].Value[0] != "0.0.0.0/0" {
		t.Fatalf("all+users: %+v", r)
	}
	// IP only, specific deny
	r = AccessListRules("bad", "any", nil, []string{"1.2.3.4/32"}, false)
	if len(r) != 1 || r[0].Action != "block" || r[0].Expression.Groups[0][0].Value[0] != "1.2.3.4/32" {
		t.Fatalf("deny only: %+v", r)
	}
}

func TestSecurityRulesStoreAndMigration(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	h := &ProxyHost{Domains: []string{"a.example.com"}, ForwardScheme: "http", ForwardHost: "app", ForwardPort: 80, Enabled: true, ClientIP: "direct"}
	if err := s.SaveProxyHost(ctx, h); err != nil {
		t.Fatal(err)
	}
	ipOnly := &AccessList{Name: "ip-only", Satisfy: "any", Allow: []string{"10.0.0.0/8"}, Deny: []string{"all"}, Users: []AccessListUser{}}
	withUsers := &AccessList{Name: "with-users", Satisfy: "any", Allow: []string{"192.168.0.0/16"}, Deny: []string{"all"},
		Users: []AccessListUser{{Username: "u", PasswordHash: "$apr1$x$y"}}}
	for _, a := range []*AccessList{ipOnly, withUsers} {
		if err := s.SaveAccessList(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	h.AccessListID = &ipOnly.ID
	if err := s.SaveProxyHost(ctx, h); err != nil {
		t.Fatal(err)
	}
	h2 := &ProxyHost{Domains: []string{"b.example.com"}, ForwardScheme: "http", ForwardHost: "app", ForwardPort: 80, Enabled: true, ClientIP: "direct", AccessListID: &withUsers.ID}
	if err := s.SaveProxyHost(ctx, h2); err != nil {
		t.Fatal(err)
	}

	n, err := s.MigrateAccessListIPRules(ctx)
	if err != nil || n != 3 { // ip-only: allow+block, with-users(any): allow
		t.Fatalf("migrate: n=%d err=%v", n, err)
	}
	if n, _ := s.MigrateAccessListIPRules(ctx); n != 0 {
		t.Fatal("migration must run once")
	}
	rules, err := s.ListSecurityRules(ctx)
	if err != nil || len(rules) != 3 {
		t.Fatalf("rules: %v %v", len(rules), err)
	}
	if rules[0].Scope != "hosts" || len(rules[0].HostIDs) != 1 || rules[0].HostIDs[0] != h.ID || rules[0].Position != 1 {
		t.Fatalf("scope: %+v", rules[0])
	}
	lists, _ := s.ListAccessLists(ctx)
	if len(lists) != 1 || lists[0].Name != "with-users" || len(lists[0].Allow) != 0 {
		t.Fatalf("access lists after migration: %+v", lists)
	}
	if hh, _ := s.ProxyHostByID(ctx, h.ID); hh.AccessListID != nil {
		t.Fatal("ip-only list must be detached")
	}

	// reorder + IP list usage
	ids := []int64{rules[2].ID, rules[0].ID, rules[1].ID}
	if err := s.ReorderSecurityRules(ctx, ids); err != nil {
		t.Fatal(err)
	}
	if err := s.ReorderSecurityRules(ctx, ids[:2]); err == nil {
		t.Fatal("partial order accepted")
	}
	got, _ := s.ListSecurityRules(ctx)
	if got[0].ID != ids[0] || got[2].ID != ids[2] {
		t.Fatal("order not applied")
	}
	l := &IPList{Name: "office", Items: []string{"10.1.0.0/16"}}
	if err := s.SaveIPList(ctx, l); err != nil {
		t.Fatal(err)
	}
	r := got[0]
	r.Expression = Expression{Groups: [][]Condition{{{Field: "ip.src", Operator: "in", Value: []string{"list:" + itoa(l.ID)}}}}}
	if err := s.SaveSecurityRule(ctx, &r); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteIPList(ctx, l.ID); err == nil {
		t.Fatal("IP list in use was deleted")
	}
	if ls, _ := s.ListIPLists(ctx); len(ls) != 1 || ls[0].Rules != 1 {
		t.Fatalf("usage: %+v", ls)
	}
	// deleting a host removes it from rule scopes (cascade)
	if err := s.DeleteProxyHost(ctx, h2.ID); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int64) string {
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
