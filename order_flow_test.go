package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignupGate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		envelope string
		status   int
		want     int
	}{
		{"accepted", `{"ok":true,"data":{},"metadata":{}}`, 200, 200},
		{"rejected envelope on 422", `{"ok":false,"error":{"code":"CAPTCHA_SCORE_TOO_LOW","message":"score below threshold"},"metadata":{}}`, 422, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/captcha/verify" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
				}
				var body struct {
					WidgetRecordID string `json:"widget_record_id"`
					Token          string `json:"token"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.WidgetRecordID != "widget-1" || body.Token != "browser-token" {
					t.Errorf("unexpected upstream body: %+v, %v", body, err)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.envelope)
			}))
			defer upstream.Close()
			s := &service{captcha: captchaClient{baseURL: upstream.URL, key: "test-key", widgetRecordID: "widget-1", http: upstream.Client()}, store: newStore()}
			r := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(`{"email":"ada@example.org","captcha_token":"browser-token"}`))
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := s.store.users["ada@example.org"]; got != (tc.want == 200) {
				t.Fatalf("registered = %v", got)
			}
		})
	}
}

func TestOrderTransitions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		signedUp  bool
		wantError bool
	}{
		{"registered customer", true, false},
		{"unregistered customer", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStore()
			if tc.signedUp {
				s.signup("ada@example.org")
			}
			o, err := s.checkout("order-17", "ada@example.org", 2599)
			if (err != nil) != tc.wantError {
				t.Fatalf("checkout error = %v", err)
			}
			if tc.wantError {
				return
			}
			if o.Receipt.AmountCents != 2599 || o.Updates[0].State != "checked_out" {
				t.Fatalf("checkout = %+v", o)
			}
			for i := 0; i < 2; i++ {
				o, err = s.fulfill("order-17")
				if err != nil || len(o.Updates) != 2 || o.Updates[1].State != "fulfilled" {
					t.Fatalf("fulfillment = %+v, %v", o, err)
				}
			}
		})
	}
}

func TestCaptchaRetryAfter(t *testing.T) {
	attempts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			io.WriteString(w, `{"ok":false,"error":{"code":"RATE_LIMITED"}}`)
			return
		}
		io.WriteString(w, `{"ok":true,"data":{}}`)
	}))
	defer upstream.Close()
	var delay time.Duration
	c := captchaClient{baseURL: upstream.URL, key: "test-key", widgetRecordID: "widget-1", http: upstream.Client(), sleep: func(_ context.Context, d time.Duration) error { delay = d; return nil }}
	if err := c.verify(context.Background(), "token", ""); err != nil || attempts != 2 || delay != 2*time.Second {
		t.Fatalf("retry: err=%v attempts=%d delay=%v", err, attempts, delay)
	}
}
