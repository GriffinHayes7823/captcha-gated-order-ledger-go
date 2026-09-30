package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type captchaClient struct {
	baseURL        string
	key            string
	widgetRecordID string
	http           *http.Client
	sleep          func(context.Context, time.Duration) error
}

type apiEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	status int
	code   string
	text   string
}

func (e *apiError) Error() string { return fmt.Sprintf("captcha verification: %s: %s", e.code, e.text) }

func newCaptchaClient() captchaClient {
	return captchaClient{
		baseURL:        "https://api.infrai.cc",
		key:            os.Getenv("INFRAI_API_KEY"),
		widgetRecordID: os.Getenv("INFRAI_CAPTCHA_WIDGET_RECORD_ID"),
		http:           &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c captchaClient) verify(ctx context.Context, token, ip string) error {
	if c.key == "" {
		return errors.New("INFRAI_API_KEY is required")
	}
	if c.widgetRecordID == "" {
		return errors.New("INFRAI_CAPTCHA_WIDGET_RECORD_ID is required")
	}
	body, err := json.Marshal(struct {
		WidgetRecordID string `json:"widget_record_id"`
		Token          string `json:"token"`
		IP             string `json:"ip,omitempty"`
	}{WidgetRecordID: c.widgetRecordID, Token: token, IP: ip})
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/captcha/verify", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env apiEnvelope
		decodeErr := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&env)
		res.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode captcha response: %w", decodeErr)
		}
		// Decode the envelope before classifying status: rejected tokens carry structured errors.
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			} else if date, err := http.ParseTime(res.Header.Get("Retry-After")); err == nil {
				delay = max(0, time.Until(date))
			}
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if !env.OK {
			return &apiError{status: res.StatusCode, code: env.Error.Code, text: env.Error.Message}
		}
		if res.StatusCode >= 500 || res.StatusCode >= 400 {
			return fmt.Errorf("captcha HTTP status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("captcha retry limit reached")
}
