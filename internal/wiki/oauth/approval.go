package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	oauthserver "github.com/go-oauth2/oauth2/v4/server"
)

type oauthApproval struct {
	UserID     string
	RequestKey string
	Details    approvalPageData
	ExpiresAt  time.Time
}

type approvalPageData struct {
	ClientLabel string
	ClientID    string
	RedirectURI string
	Scope       string
	Resource    string
}

func (s *Service) issueApproval(userID, requestKey string, details approvalPageData) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("create oauth approval token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])

	now := time.Now()
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	for existing, approval := range s.approvals {
		if !approval.ExpiresAt.After(now) {
			delete(s.approvals, existing)
		}
	}
	s.approvals[token] = oauthApproval{
		UserID:     userID,
		RequestKey: requestKey,
		Details:    details,
		ExpiresAt:  now.Add(approvalTTL),
	}
	return token, nil
}

func (s *Service) consumeApproval(token, userID, requestKey string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	approval, ok := s.approvals[token]
	if !ok {
		return false
	}
	delete(s.approvals, token)
	return approval.UserID == userID && approval.RequestKey == requestKey && approval.ExpiresAt.After(time.Now())
}

func (s *Service) approvalDetails(token, userID string) (approvalPageData, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return approvalPageData{}, false
	}

	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	approval, ok := s.approvals[token]
	if !ok {
		return approvalPageData{}, false
	}
	if !approval.ExpiresAt.After(time.Now()) {
		delete(s.approvals, token)
		return approvalPageData{}, false
	}
	if approval.UserID != userID {
		return approvalPageData{}, false
	}
	return approval.Details, true
}

func (s *Service) approvalPageData(req *http.Request, ar *oauthserver.AuthorizeRequest, basePath string) approvalPageData {
	client, _ := s.client(ar.ClientID)
	label := strings.TrimSpace(client.ClientName)
	if label == "" {
		label = ar.ClientID
	}
	scope := strings.TrimSpace(ar.Scope)
	if scope == "" {
		scope = ScopeMCP
	}
	resource := strings.TrimSpace(req.FormValue("resource"))
	if resource == "" {
		resource = MCPResourceURL(req, basePath)
	}
	return approvalPageData{
		ClientLabel: label,
		ClientID:    ar.ClientID,
		RedirectURI: ar.RedirectURI,
		Scope:       scope,
		Resource:    resource,
	}
}

func authorizeApprovalValues(req *http.Request) (url.Values, string, error) {
	if err := req.ParseForm(); err != nil {
		return nil, "", err
	}
	values := url.Values{}
	for _, key := range []string{
		"client_id",
		"response_type",
		"redirect_uri",
		"scope",
		"state",
		"code_challenge",
		"code_challenge_method",
		"resource",
	} {
		for _, value := range req.Form[key] {
			values.Add(key, value)
		}
	}
	return values, values.Encode(), nil
}
