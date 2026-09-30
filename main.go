package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

type service struct {
	captcha captchaClient
	store   *store
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(target)
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Email        string `json:"email"`
			CaptchaToken string `json:"captcha_token"`
		}
		if readJSON(r, &in) != nil || !strings.Contains(in.Email, "@") || in.CaptchaToken == "" {
			respond(w, 400, map[string]string{"error": "email and captcha_token required"})
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if err := s.captcha.verify(r.Context(), in.CaptchaToken, ip); err != nil {
			var apiErr *apiError
			if errors.As(err, &apiErr) && apiErr.status >= 400 && apiErr.status < 500 && apiErr.status != 429 {
				respond(w, 403, map[string]string{"error": apiErr.code})
			} else {
				respond(w, 503, map[string]string{"error": "verification unavailable"})
			}
			return
		}
		s.store.signup(in.Email)
		respond(w, 200, map[string]string{"email": in.Email, "state": "registered"})
	})
	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			OrderID     string `json:"order_id"`
			Email       string `json:"email"`
			AmountCents int64  `json:"amount_cents"`
		}
		if readJSON(r, &in) != nil || in.OrderID == "" || in.Email == "" || in.AmountCents <= 0 {
			respond(w, 400, map[string]string{"error": "order_id, email and positive amount_cents required"})
			return
		}
		o, err := s.store.checkout(in.OrderID, in.Email, in.AmountCents)
		if err != nil {
			respond(w, 409, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, o)
	})
	mux.HandleFunc("POST /orders/{id}/fulfill", func(w http.ResponseWriter, r *http.Request) {
		o, err := s.store.fulfill(r.PathValue("id"))
		if err != nil {
			respond(w, 404, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, o)
	})
	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		o, ok := s.store.get(r.PathValue("id"))
		if !ok {
			respond(w, 404, map[string]string{"error": "order not found"})
			return
		}
		respond(w, 200, o)
	})
	return mux
}

func main() {
	if os.Getenv("INFRAI_API_KEY") == "" {
		log.Fatal("set INFRAI_API_KEY")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, (&service{captcha: newCaptchaClient(), store: newStore()}).routes()))
}
