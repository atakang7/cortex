# ratelimit

Token-bucket rate limiting.

```python
from ratelimit import TokenBucket

bucket = TokenBucket(rate=5, capacity=10)   # 5 per second, bursts of 10
if bucket.allow():
    handle(request)
```

A bucket starts full. Every call to `allow(n)` takes `n` tokens if there are
that many and returns True; otherwise it takes none and returns False. Tokens
come back at `rate` per second, up to `capacity`.

Pass `clock=` (a function returning seconds) to control time in tests;
`ratelimit.clock.FakeClock` is one.

Tests: `python3 -m unittest discover -s tests`
