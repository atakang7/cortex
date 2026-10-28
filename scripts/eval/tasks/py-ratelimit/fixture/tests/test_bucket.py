import unittest

from ratelimit import TokenBucket
from ratelimit.clock import FakeClock


class TokenBucketTest(unittest.TestCase):
    def test_starts_full_and_drains(self):
        clock = FakeClock()
        b = TokenBucket(rate=1, capacity=3, clock=clock)
        self.assertEqual([b.allow() for _ in range(4)], [True, True, True, False])

    def test_refills_over_time(self):
        clock = FakeClock()
        b = TokenBucket(rate=1, capacity=3, clock=clock)
        for _ in range(3):
            b.allow()
        clock.advance(2)
        self.assertTrue(b.allow())
        self.assertTrue(b.allow())
        self.assertFalse(b.allow())

    def test_never_above_capacity(self):
        clock = FakeClock()
        b = TokenBucket(rate=10, capacity=2, clock=clock)
        clock.advance(60)
        self.assertEqual(b.tokens, 2)

    def test_takes_n(self):
        b = TokenBucket(rate=1, capacity=5, clock=FakeClock())
        self.assertTrue(b.allow(4))
        self.assertFalse(b.allow(2))
        self.assertTrue(b.allow(1))

    def test_rejects_bad_arguments(self):
        with self.assertRaises(ValueError):
            TokenBucket(rate=0, capacity=1)
        with self.assertRaises(ValueError):
            TokenBucket(rate=1, capacity=0)
        with self.assertRaises(ValueError):
            TokenBucket(rate=1, capacity=1).allow(0)


if __name__ == "__main__":
    unittest.main()
