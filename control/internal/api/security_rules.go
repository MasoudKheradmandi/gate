package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"corvus/control/internal/geoip"
	"corvus/control/internal/store"
	"corvus/control/internal/validate"
)

// GeoIPService is the country database used by security rules (internal/geoip).
type GeoIPService interface {
	Status() geoip.Status
	Refresh(ctx context.Context) (geoip.Status, error)
}

type ruleInput struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Enabled       *bool            `json:"enabled"`
	Action        string           `json:"action"`
	StatusCode    int              `json:"status_code"`
	SkipBasicAuth bool             `json:"skip_basic_auth"`
	Scope         string           `json:"scope"`
	HostIDs       []int64          `json:"host_ids"`
	Expression    store.Expression `json:"expression"`
	// read-only fields the UI may send back unchanged
	ID        int64      `json:"id"`
	Position  int        `json:"position"`
	Hits24h   int64      `json:"hits_24h"`
	LastHitAt *time.Time `json:"last_hit_at"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (in ruleInput) build(ctx context.Context, tx *store.Store, r *store.SecurityRule) error {
	r.Name, r.Description, r.Action, r.StatusCode, r.SkipBasicAuth = in.Name, in.Description, in.Action, in.StatusCode, in.SkipBasicAuth
	r.Scope, r.HostIDs, r.Expression = in.Scope, in.HostIDs, in.Expression
	if in.Enabled != nil {
		r.Enabled = *in.Enabled
	} else if r.ID == 0 {
		r.Enabled = true
	}
	lists, err := tx.ListIPLists(ctx)
	if err != nil {
		return err
	}
	listIDs := map[int64]bool{}
	for _, l := range lists {
		listIDs[l.ID] = true
	}
	hosts, err := tx.ListProxyHosts(ctx)
	if err != nil {
		return err
	}
	hostIDs := map[int64]bool{}
	for _, h := range hosts {
		hostIDs[h.ID] = true
	}
	return validate.SecurityRule(r, func(id int64) bool { return listIDs[id] }, func(id int64) bool { return hostIDs[id] })
}

func (s *Server) withHits(list []store.SecurityRule) {
	if s.Traffic == nil {
		return
	}
	hits := s.Traffic.RuleHits(time.Now())
	for i := range list {
		if h, ok := hits[list[i].ID]; ok {
			list[i].Hits24h, list[i].LastHitAt = h.Hits24h, h.Last
		}
	}
}

func (s *Server) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSecurityRules(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	s.withHits(list)
	writeJSON(w, 200, list)
}

func (s *Server) createSecurityRule(w http.ResponseWriter, r *http.Request) {
	var in ruleInput
	if !decode(w, r, &in) {
		return
	}
	rule := &store.SecurityRule{}
	v, err := s.Deployer.Apply(r.Context(), userID(r), "create security rule "+in.Name, func(tx *store.Store) error {
		list, err := tx.ListSecurityRules(r.Context())
		if err != nil {
			return err
		}
		if len(list) >= validate.MaxRules {
			return conflictError{fmt.Sprintf("at most %d security rules", validate.MaxRules)}
		}
		if err := in.build(r.Context(), tx, rule); err != nil {
			return err
		}
		return tx.SaveSecurityRule(r.Context(), rule)
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.audit(r, "security_rule.create", fmt.Sprintf("security_rule:%d", rule.ID), rule)
	writeJSON(w, 201, map[string]any{"rule": rule, "version": v})
}

func (s *Server) updateSecurityRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in ruleInput
	if !decode(w, r, &in) {
		return
	}
	var before, rule *store.SecurityRule
	v, err := s.Deployer.Apply(r.Context(), userID(r), fmt.Sprintf("update security rule #%d", id), func(tx *store.Store) error {
		var err error
		if before, err = tx.SecurityRuleByID(r.Context(), id); err != nil {
			return err
		}
		cp := *before
		rule = &cp
		if err := in.build(r.Context(), tx, rule); err != nil {
			return err
		}
		return tx.SaveSecurityRule(r.Context(), rule)
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.audit(r, "security_rule.update", fmt.Sprintf("security_rule:%d", id), map[string]any{"before": before, "after": rule})
	writeJSON(w, 200, map[string]any{"rule": rule, "version": v})
}

func (s *Server) deleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var before *store.SecurityRule
	v, err := s.Deployer.Apply(r.Context(), userID(r), fmt.Sprintf("delete security rule #%d", id), func(tx *store.Store) error {
		var err error
		if before, err = tx.SecurityRuleByID(r.Context(), id); err != nil {
			return err
		}
		return tx.DeleteSecurityRule(r.Context(), id)
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.audit(r, "security_rule.delete", fmt.Sprintf("security_rule:%d", id), before)
	writeJSON(w, 200, map[string]any{"version": v})
}

func (s *Server) reorderSecurityRules(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := s.Deployer.Apply(r.Context(), userID(r), "reorder security rules", func(tx *store.Store) error {
		if err := tx.ReorderSecurityRules(r.Context(), in.IDs); err != nil {
			return validate.Errors{{Field: "ids", Code: "invalid", Message: err.Error()}}
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.audit(r, "security_rule.reorder", "security_rules", in.IDs)
	writeJSON(w, 200, map[string]any{"version": v})
}

func (s *Server) securityRuleEvents(w http.ResponseWriter, r *http.Request) {
	if s.Traffic == nil {
		writeJSON(w, 200, []any{})
		return
	}
	ruleID, _ := strconv.ParseInt(r.URL.Query().Get("rule_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events := s.Traffic.RuleEvents(ruleID, limit)
	if rules, err := s.Store.ListSecurityRules(r.Context()); err == nil {
		names := map[int64]string{}
		for _, x := range rules {
			names[x.ID] = x.Name
		}
		for i := range events {
			if events[i].RuleID != 0 {
				events[i].Rule = names[events[i].RuleID]
			}
		}
	}
	writeJSON(w, 200, events)
}

func (s *Server) geoipStatus(w http.ResponseWriter, r *http.Request) {
	if s.GeoIP == nil {
		writeJSON(w, 200, geoip.Status{Attribution: geoip.Attribution, Disabled: true})
		return
	}
	writeJSON(w, 200, s.GeoIP.Status())
}

func (s *Server) geoipRefresh(w http.ResponseWriter, r *http.Request) {
	if s.GeoIP == nil {
		writeErr(w, http.StatusConflict, "conflict", "country database is not available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	st, err := s.GeoIP.Refresh(ctx)
	s.audit(r, "geoip.refresh", "geoip", map[string]any{"ok": err == nil, "source": st.Source})
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "geoip_failed", "message": err.Error(), "status": st})
		return
	}
	writeJSON(w, 200, st)
}

// ------------------------------------------------------------------ IP lists

type ipListInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Items       []string   `json:"items"`
	ID          int64      `json:"id"`
	Rules       int        `json:"rules"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// decodeLarge is decode with a larger body limit (IP lists may hold 50,000 entries).
func decodeLarge(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func (s *Server) listIPLists(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListIPLists(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) saveIPList(w http.ResponseWriter, r *http.Request, id int64) {
	var in ipListInput
	if !decodeLarge(w, r, &in) {
		return
	}
	l := &store.IPList{ID: id, Name: in.Name, Description: in.Description, Items: in.Items}
	if l.Items == nil {
		l.Items = []string{}
	}
	msg := "create IP list " + in.Name
	if id != 0 {
		msg = fmt.Sprintf("update IP list #%d", id)
	}
	v, err := s.Deployer.Apply(r.Context(), userID(r), msg, func(tx *store.Store) error {
		if id != 0 {
			if _, err := tx.IPListByID(r.Context(), id); err != nil {
				return err
			}
		}
		if err := validate.IPList(l); err != nil {
			return err
		}
		if err := tx.SaveIPList(r.Context(), l); err != nil {
			if store.IsUnique(err) {
				return validate.Errors{{Field: "name", Code: "duplicate", Message: "an IP list with this name already exists"}}
			}
			return err
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	code, action := 201, "ip_list.create"
	if id != 0 {
		code, action = 200, "ip_list.update"
	}
	s.audit(r, action, fmt.Sprintf("ip_list:%d", l.ID), map[string]any{"name": l.Name, "items": len(l.Items)})
	writeJSON(w, code, map[string]any{"ip_list": l, "version": v})
}

func (s *Server) createIPList(w http.ResponseWriter, r *http.Request) { s.saveIPList(w, r, 0) }

func (s *Server) updateIPList(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r); ok {
		s.saveIPList(w, r, id)
	}
}

func (s *Server) deleteIPList(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, err := s.Deployer.Apply(r.Context(), userID(r), fmt.Sprintf("delete IP list #%d", id), func(tx *store.Store) error {
		if _, err := tx.IPListByID(r.Context(), id); err != nil {
			return err
		}
		if err := tx.DeleteIPList(r.Context(), id); err != nil {
			return conflictError{err.Error()}
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.audit(r, "ip_list.delete", fmt.Sprintf("ip_list:%d", id), nil)
	writeJSON(w, 200, map[string]any{"version": v})
}
