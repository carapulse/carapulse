package web

import (
	"errors"
	"net/http"
	"strings"
)

func (s *Server) policyCheckRead(r *http.Request, action string) error {
	if s != nil && s.SessionRequired {
		if _, err := s.requireSession(r); err != nil {
			return err
		}
	}
	ctxRef := contextFromHeaders(r)
	if hasContextValues(ctxRef) {
		if err := validateContextRefStrict(ctxRef); err != nil {
			return err
		}
	} else if s != nil && s.SessionRequired {
		return errors.New("tenant_id required")
	}
	return s.policyCheck(r, action, "read", ctxRef, "read", 0)
}

func (s *Server) policyCheckReadPlan(r *http.Request, plan map[string]any, action string) (ContextRef, error) {
	if err := s.enforceSessionRequired(r, plan); err != nil {
		return ContextRef{}, err
	}
	ctxRef := ContextRef{}
	if ctxVal, ok := plan["context"].(map[string]any); ok {
		ctxRef = contextFromMap(ctxVal)
	}
	if err := validateContextRefStrict(ctxRef); err != nil {
		return ContextRef{}, err
	}
	if err := s.policyCheck(r, action, "read", ctxRef, "read", 0); err != nil {
		return ContextRef{}, err
	}
	return ctxRef, nil
}

func (s *Server) enforceSessionRequired(r *http.Request, plan map[string]any) error {
	if s != nil && s.SessionRequired {
		if _, err := s.requireSession(r); err != nil {
			return err
		}
	}
	return enforceSessionMatch(r, plan)
}

func contextFromHeaders(r *http.Request) ContextRef {
	if r == nil {
		return ContextRef{}
	}
	return ContextRef{
		TenantID:      strings.TrimSpace(r.Header.Get("X-Tenant-Id")),
		Environment:   strings.TrimSpace(r.Header.Get("X-Environment")),
		ClusterID:     strings.TrimSpace(r.Header.Get("X-Cluster-Id")),
		Namespace:     strings.TrimSpace(r.Header.Get("X-Namespace")),
		AWSAccountID:  strings.TrimSpace(r.Header.Get("X-AWS-Account-Id")),
		Region:        strings.TrimSpace(r.Header.Get("X-Region")),
		ArgoCDProject: strings.TrimSpace(r.Header.Get("X-ArgoCD-Project")),
		GrafanaOrgID:  strings.TrimSpace(r.Header.Get("X-Grafana-Org-Id")),
	}
}

func hasContextValues(ctxRef ContextRef) bool {
	return ctxRef.TenantID != "" ||
		ctxRef.Environment != "" ||
		ctxRef.ClusterID != "" ||
		ctxRef.Namespace != "" ||
		ctxRef.AWSAccountID != "" ||
		ctxRef.Region != "" ||
		ctxRef.ArgoCDProject != "" ||
		ctxRef.GrafanaOrgID != ""
}
