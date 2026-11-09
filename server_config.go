package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type serverOptions struct {
	AgentToken string
	ReadOnly   bool
	Seed       string
}

func agentToken(path string) (string, error) {
	if token := strings.TrimSpace(os.Getenv("PANL_AGENT_TOKEN")); token != "" {
		if len(token) < 24 {
			return "", errors.New("PANL_AGENT_TOKEN must be at least 24 characters")
		}
		return token, nil
	}
	if b, e := os.ReadFile(path); e == nil {
		token := strings.TrimSpace(string(b))
		if len(token) < 24 {
			return "", errors.New("agent token file must contain at least 24 characters")
		}
		return token, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", e
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	token := hex.EncodeToString(b)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return agentToken(path)
	}
	if e != nil {
		return "", e
	}
	if _, e = f.WriteString(token + "\n"); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	return token, nil
}
func (s *dashboardServer) authorized(r *http.Request) bool {
	token, hasBearer := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return hasBearer && s.agentToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.agentToken)) == 1
}
func (s *dashboardServer) protectAgent(w http.ResponseWriter, r *http.Request) bool {
	if s.readOnly {
		apiError(w, http.StatusForbidden, errors.New("this instance is read-only"))
		return false
	}
	if !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="panl-agent"`)
		apiError(w, http.StatusUnauthorized, errors.New("agent bearer token required"))
		return false
	}
	return true
}

// Reject trailing JSON: malformed multi-object requests must not partially mutate state.
func requireEOF(dec interface{ Decode(any) error }) error {
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		return fmt.Errorf("request must contain exactly one JSON value")
	}
	return nil
}
