package api

import (
	"context"
	"fmt"
	"strings"

	"corvus/control/internal/auth"
	"corvus/control/internal/store"
	"corvus/control/internal/validate"
)

// Per-host password protection (HTTP Basic auth), edited inside the proxy host form.
// Each host gets its own list named "host-<id>"; a list shared by several hosts (for
// example from an NPM import) is copied on first change so other hosts are unaffected.

type basicAuthInput struct {
	Enabled bool `json:"enabled"`
	Users   []struct {
		Username string `json:"username"`
		Password string `json:"password"` // empty = keep the current password
	} `json:"users"`
}

func ownListName(hostID int64) string { return fmt.Sprintf("host-%d", hostID) }

// applyBasicAuth runs inside the deploy transaction after the host was saved (h.ID set).
func applyBasicAuth(ctx context.Context, tx *store.Store, h *store.ProxyHost, before *store.ProxyHost, in *basicAuthInput) error {
	var current *store.AccessList
	if h.AccessListID != nil {
		current, _ = tx.AccessListByID(ctx, *h.AccessListID)
	} else if before != nil && before.AccessListID != nil {
		current, _ = tx.AccessListByID(ctx, *before.AccessListID)
	}
	own := current != nil && current.Name == ownListName(h.ID)

	if !in.Enabled || len(in.Users) == 0 {
		if in.Enabled {
			return validate.Errors{{Field: "basic_auth", Code: "empty", Message: "add at least one user"}}
		}
		h.AccessListID = nil
		if err := tx.SaveProxyHost(ctx, h); err != nil {
			return err
		}
		if own {
			_ = tx.DeleteAccessList(ctx, current.ID) // refuses while another host still uses it
		}
		return nil
	}

	old := map[string]string{}
	if current != nil {
		for _, u := range current.Users {
			old[u.Username] = u.PasswordHash
		}
	}
	list := &store.AccessList{Name: ownListName(h.ID)}
	if own {
		list.ID, list.CreatedAt = current.ID, current.CreatedAt
	}
	for _, u := range in.Users {
		name := strings.TrimSpace(u.Username)
		hash := old[name]
		if u.Password != "" {
			if len(u.Password) < 8 {
				return validate.Errors{{Field: "basic_auth", Code: "weak", Message: "passwords must be at least 8 characters"}}
			}
			if len(u.Password) > 72 {
				return validate.Errors{{Field: "basic_auth", Code: "too_long", Message: "passwords must be at most 72 bytes"}}
			}
			h, err := auth.HtpasswdHash(u.Password)
			if err != nil {
				return err
			}
			hash = h
		}
		if hash == "" {
			return validate.Errors{{Field: "basic_auth", Code: "password_required", Message: fmt.Sprintf("a password is required for new user %q", name)}}
		}
		list.Users = append(list.Users, store.AccessListUser{Username: name, PasswordHash: hash})
	}
	if err := validate.AccessList(list); err != nil {
		var out validate.Errors
		for _, e := range err.(validate.Errors) {
			e.Field = "basic_auth"
			out = append(out, e)
		}
		return out
	}
	if err := tx.SaveAccessList(ctx, list); err != nil {
		return err
	}
	h.AccessListID = &list.ID
	return tx.SaveProxyHost(ctx, h)
}

// withAuthUsers fills the read-only basic_auth_users field (usernames only).
func (s *Server) withAuthUsers(ctx context.Context, hosts []store.ProxyHost) {
	lists, err := s.Store.ListAccessLists(ctx)
	if err != nil {
		return
	}
	byID := map[int64][]string{}
	for _, l := range lists {
		names := []string{}
		for _, u := range l.Users {
			names = append(names, u.Username)
		}
		byID[l.ID] = names
	}
	for i := range hosts {
		hosts[i].BasicAuthUsers = []string{}
		if hosts[i].AccessListID != nil {
			if n, ok := byID[*hosts[i].AccessListID]; ok {
				hosts[i].BasicAuthUsers = n
			}
		}
	}
}
