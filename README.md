# hello-world

Sample app for the Arzana e2e workflow. Demonstrates the three platform
env vars (`ARZANA_ENV`, `ARZANA_KV_URL`, `ARZANA_SERVICE`) plus every
configuration kind:

- `FOO` — plain env var, set by the user.
- `BAR` — env-bound secret, resolved by the platform at deploy time.
- `STRIPE_KEY` — SDK-only secret, never injected as an env var; the app
  fetches it via `azsecrets` using `ARZANA_KV_URL` and the runtime's
  managed identity.

## Endpoints

- `GET /healthz` — returns `200 {"status":"ok"}`. Used by the deploy
  saga's health probe.
- `GET /check-secrets` — returns a JSON object echoing every value the
  app sees, so the e2e test can assert all three resolution paths
  landed the values it set:

  ```json
  {
    "foo": "...",
    "bar": "...",
    "stripe": "...",
    "env": "prod",
    "service": "hello-world",
    "kv_url": "https://arz-...-prod-kv.vault.azure.net/",
    "sdk_err": ""
  }
  ```

  This deliberately echoes secret values — it is a test fixture, not a
  security exemplar. Never copy this pattern into a real service.

## Build

```sh
docker build -t hello-world examples/hello-world
docker run --rm -p 8080:8080 -e FOO=hello hello-world
```

The e2e harness pushes this image into the user's sandbox subscription's
ACR and deploys it through the standard Arzana flow.
