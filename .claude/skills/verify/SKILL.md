---
name: verify-webhook-relay
---

# Verify webhook relay

```powershell
# Build temporary relay and fake Lark executables, then launch fake Lark on 19999
# and Gh2Lark on 18080 with LARK_WEBHOOK_URL=http://127.0.0.1:19999.
# Set GITHUB_WEBHOOK_SECRET, calculate sha256=<hex> HMAC for a sample payload,
# POST it to /webhook with X-GitHub-Event, then inspect fake Lark's received card.
# Also send the same payload with an invalid signature and expect HTTP 401.
```

Drive at least one event type changed by the diff and confirm the fake receiver observes the expected interactive-card title, details, color, and button URL. Clean temporary executables, logs, and listener processes afterward.
