"""Hidden acceptance tests: never shown to the agent."""

import sys
import threading
import unittest

from ratelimit import TokenBucket


class Clock:
    def __init__(self):
        self.now = 1000.0

    def __call__(self):
        return self.now

    def advance(self, s):
        self.now += s


class HiddenBucket(unittest.TestCase):
    def test_hidden_steady_traffic_is_not_starved(self):
        c = Clock()
        b = TokenBucket(rate=2, capacity=2, clock=c)
        self.assertTrue(b.allow())
        self.assertTrue(b.allow())
        allowed = 0
        for _ in range(20):  # 6 seconds at one call per 300 ms: about 12 should pass
            c.advance(0.3)
            allowed += b.allow()
        self.assertGreaterEqual(allowed, 11)
        self.assertLessEqual(allowed, 13)

    def test_hidden_fractions_accumulate(self):
        c = Clock()
        b = TokenBucket(rate=2, capacity=2, clock=c)
        b.allow(2)
        c.advance(0.25)
        self.assertFalse(b.allow())
        c.advance(0.25)
        self.assertTrue(b.allow())

    def test_hidden_capacity_still_caps(self):
        c = Clock()
        b = TokenBucket(rate=5, capacity=2, clock=c)
        c.advance(100)
        self.assertEqual([b.allow(), b.allow(), b.allow()], [True, True, False])


class HiddenKeyed(unittest.TestCase):
    def setUp(self):
        global KeyedLimiter
        from ratelimit import KeyedLimiter

    def test_hidden_keys_are_independent(self):
        c = Clock()
        k = KeyedLimiter(rate=1, capacity=1, idle_ttl=10, clock=c)
        self.assertTrue(k.allow("a"))
        self.assertFalse(k.allow("a"))
        self.assertTrue(k.allow("b"))
        self.assertTrue(k.allow("c", n=1))

    def test_hidden_n_tokens(self):
        k = KeyedLimiter(rate=1, capacity=5, idle_ttl=10, clock=Clock())
        self.assertTrue(k.allow("a", 4))
        self.assertFalse(k.allow("a", 2))
        self.assertTrue(k.allow("a"))

    def test_hidden_idle_buckets_are_dropped(self):
        c = Clock()
        k = KeyedLimiter(rate=1, capacity=1, idle_ttl=10, clock=c)
        k.allow("a")
        c.advance(5)
        k.allow("b")
        self.assertEqual(len(k), 2)
        c.advance(6)  # a idle 11 s, b idle 6 s
        k.allow("c")
        self.assertEqual(len(k), 2)
        c.advance(20)
        k.allow("d")
        self.assertEqual(len(k), 1)

    def test_hidden_idle_means_since_last_use(self):
        c = Clock()
        k = KeyedLimiter(rate=0.01, capacity=1, idle_ttl=10, clock=c)
        self.assertTrue(k.allow("a"))
        c.advance(6)
        self.assertFalse(k.allow("a"))
        c.advance(6)  # 12 s since the bucket was made, 6 s since its last use
        self.assertFalse(k.allow("a"), "a bucket in use was dropped and came back full")
        self.assertEqual(len(k), 1)

    def test_hidden_thread_safe(self):
        old = sys.getswitchinterval()
        sys.setswitchinterval(1e-6)
        try:
            for _ in range(5):  # a race shows up only sometimes; five rounds
                k = KeyedLimiter(rate=0.001, capacity=100, idle_ttl=60, clock=Clock())
                counts = {f"k{i}": 0 for i in range(4)}
                lock = threading.Lock()

                def worker():
                    for _ in range(150):
                        for key in counts:
                            if k.allow(key):
                                with lock:
                                    counts[key] += 1

                threads = [threading.Thread(target=worker) for _ in range(8)]
                for t in threads:
                    t.start()
                for t in threads:
                    t.join()
                self.assertEqual(counts, {key: 100 for key in counts})
        finally:
            sys.setswitchinterval(old)


if __name__ == "__main__":
    unittest.main()
