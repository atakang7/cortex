"""A single token bucket."""

import time


class TokenBucket:
    """Allows `rate` events per second on average, in bursts of up to `capacity`."""

    def __init__(self, rate, capacity, clock=time.monotonic):
        if rate <= 0:
            raise ValueError("rate must be positive")
        if capacity < 1:
            raise ValueError("capacity must be at least 1")
        self.rate = rate
        self.capacity = capacity
        self._clock = clock
        self._tokens = capacity
        self._last = clock()

    def _refill(self):
        now = self._clock()
        added = int((now - self._last) * self.rate)
        self._tokens = min(self.capacity, self._tokens + added)
        self._last = now

    @property
    def tokens(self):
        """Tokens available right now."""
        self._refill()
        return self._tokens

    def allow(self, n=1):
        """Take n tokens if available. Returns whether they were taken."""
        if n < 1:
            raise ValueError("n must be at least 1")
        self._refill()
        if self._tokens >= n:
            self._tokens -= n
            return True
        return False
