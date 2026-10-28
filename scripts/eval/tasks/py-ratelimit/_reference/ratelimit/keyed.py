"""Reference solution for validating the oracle only; never shown to the agent."""

import threading
import time

from ratelimit.bucket import TokenBucket


class KeyedLimiter:
    def __init__(self, rate, capacity, idle_ttl, clock=time.monotonic):
        if idle_ttl <= 0:
            raise ValueError("idle_ttl must be positive")
        self.rate, self.capacity, self.idle_ttl = rate, capacity, idle_ttl
        self._clock = clock
        self._lock = threading.Lock()
        self._buckets = {}  # key -> (bucket, last use)

    def allow(self, key, n=1):
        with self._lock:
            now = self._clock()
            for k in [k for k, (_, last) in self._buckets.items() if now - last > self.idle_ttl]:
                del self._buckets[k]
            entry = self._buckets.get(key)
            bucket = entry[0] if entry else TokenBucket(self.rate, self.capacity, self._clock)
            self._buckets[key] = (bucket, now)
            return bucket.allow(n)

    def __len__(self):
        with self._lock:
            return len(self._buckets)
