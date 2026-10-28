Users report that our rate limiter starves under steady traffic: with `TokenBucket(rate=2, capacity=2)`, calling `allow()` every 300 ms denies every call once the initial burst is spent, although 2 per second should get through. Find and fix the cause.

Then add `KeyedLimiter(rate, capacity, idle_ttl, clock=time.monotonic)` to the package, importable as `from ratelimit import KeyedLimiter`:

- `allow(key, n=1)` limits each key with its own bucket of the given rate and capacity.
- A key's bucket is dropped once it has been idle (no `allow` for that key) for longer than `idle_ttl` seconds, at the latest on the next call to `allow` for any key, so memory stays bounded. `len(limiter)` is the number of buckets currently kept.
- It is called from many threads at once, so it must be thread-safe.

Add tests for both, and document KeyedLimiter in the README.
