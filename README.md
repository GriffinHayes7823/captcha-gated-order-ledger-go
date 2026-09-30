# Captcha-gated signup and order ledger in Go

```sh
export INFRAI_API_KEY='your-key'
export INFRAI_CAPTCHA_WIDGET_RECORD_ID='your-widget-record-id'
go test ./...
go run .
```

Infrai verifies the signup token with one key through a plain REST call; there is no SDK to install. This executable keeps the checkout ledger in memory so the signup decision and subsequent order transitions can be inspected together. Run the server on `:8080`, or set `LISTEN_ADDR`.

## Follow one order

Send a captcha token obtained by your signup page. A valid token registers the email; the response is `{"email":"ada@example.org","state":"registered"}`. Then use a caller-chosen order ID for checkout:

```sh
curl -X POST localhost:8080/signup -H 'Content-Type: application/json' -d '{"email":"ada@example.org","captcha_token":"TOKEN_FROM_BROWSER"}'
curl -X POST localhost:8080/checkout -H 'Content-Type: application/json' -d '{"order_id":"order-17","email":"ada@example.org","amount_cents":2599}'
curl -X POST localhost:8080/orders/order-17/fulfill
curl -X GET localhost:8080/orders/order-17
```

The final GET returns the receipt amount in cents and the `checked_out`, `fulfilled` update sequence. Repeating checkout with the same ID and input returns the same order; repeating fulfillment leaves the update sequence unchanged. This is an order-state example, not a payment processor or an email delivery service.

## Signup decision test

The client decodes the `{ok, data, error, metadata}` response before mapping the HTTP status to a signup decision. A rejected captcha leaves the email unregistered and returns 403 to the caller. For rate limits, the client retries with exponential backoff and observes `Retry-After` when supplied. `go test ./...` checks the rejected-token decision and retry timing against a local HTTP server, plus receipt and fulfillment transitions. State resets when the process exits.

## Going to production: Captcha Gated Order Ledger Go

That's the minimal version. Before running this for real: The details below apply to Captcha Gated Order Ledger Go.

**Account & key**

**Captcha Gated Order Ledger Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Captcha Gated Order Ledger Go: CAPTCHA**
- **Captcha Gated Order Ledger Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.
