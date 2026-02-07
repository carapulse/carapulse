package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type PolicyService struct {
	OPAURL        string
	PolicyPackage string
	HTTPClient    *http.Client
	clientOnce    sync.Once
}

type PolicyInput struct {
	Actor     any   `json:"actor"`
	Action    any   `json:"action"`
	Context   any   `json:"context"`
	Resources any   `json:"resources"`
	Risk      any   `json:"risk"`
	Time      string `json:"time"`
}

type PolicyDecision struct {
	Decision    string         `json:"decision"`
	Constraints map[string]any `json:"constraints"`
	TTL         int            `json:"ttl"`
}

type opaResponse struct {
	Result PolicyDecision `json:"result"`
}

func (p *PolicyService) httpClient() *http.Client {
	p.clientOnce.Do(func() {
		if p.HTTPClient == nil {
			p.HTTPClient = &http.Client{Timeout: 5 * time.Second}
		}
	})
	return p.HTTPClient
}

func (p *PolicyService) Evaluate(input PolicyInput) (PolicyDecision, error) {
	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return PolicyDecision{}, err
	}
	pkg := strings.Trim(strings.TrimSpace(p.PolicyPackage), "/")
	pkg = strings.ReplaceAll(pkg, ".", "/")
	base := strings.TrimRight(strings.TrimSpace(p.OPAURL), "/")
	url := fmt.Sprintf("%s/v1/data/%s", base, pkg)
	resp, err := p.httpClient().Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return PolicyDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PolicyDecision{}, fmt.Errorf("opa status %d", resp.StatusCode)
	}
	var out opaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PolicyDecision{}, err
	}
	return out.Result, nil
}
