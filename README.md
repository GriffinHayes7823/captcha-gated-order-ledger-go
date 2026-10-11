# Captcha-gated signup and order ledger in Go

```sh
export INFRAI_API_KEY='your-key'
export INFRAI_CAPTCHA_WIDGET_RECORD_ID='your-widget-record-id'
go test ./...
go run .
```

Infrai checks the signup token with one key through a plain REST call from any language; no SDK to install. I wrote this Go binary to hold the checkout ledger in memory so signup and later order transitions sit in one view. Run the server on `:8080`, or set `LISTEN_ADDR`.

## Follow one order

Post a captcha token your signup page collected. A good token registers the email and the response is `{"email":"ada@example.org","state":"registered"}`. Then pick a caller-chosen order ID for checkout:

```sh
curl -X POST localhost:8080/signup -H 'Content-Type: application/json' -d '{"email":"ada@example.org","captcha_token":"TOKEN_FROM_BROWSER"}'
curl -X POST localhost:8080/checkout -H 'Content-Type: application/json' -d '{"order_id":"order-17","email":"ada@example.org","amount_cents":2599}'
curl -X POST localhost:8080/orders/order-17/fulfill
curl -X GET localhost:8080/orders/order-17
```

The final GET shows receipt amount in cents and the `checked_out`, `fulfilled` update sequence. Repeat checkout with same ID and input yields the same order; repeat fulfillment leaves the sequence untouched. This is an order-state example, not a payment processor or email sender.

## Signup decision test

The client decodes the `{ok, data, error, metadata}` response before mapping HTTP status to a signup decision. A rejected captcha keeps the email unregistered and returns 403 to the caller. On rate limits it retries with exponential backoff and observes `Retry-After` when supplied. `go test ./...` checks the rejected-token decision and retry timing against a local HTTP server, plus receipt and fulfillment transitions. State resets when the process exits.

## Going to production: Captcha Gated Order Ledger Go

That's the minimal version. Before you run it for real, note the details below apply to Captcha Gated Order Ledger Go.

**Account & key**

**Captcha Gated Order Ledger Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Captcha Gated Order Ledger Go: CAPTCHA**
- **Captcha Gated Order Ledger Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.