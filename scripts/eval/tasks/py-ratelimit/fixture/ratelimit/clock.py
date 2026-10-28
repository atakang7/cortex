"""Clocks for tests."""


class FakeClock:
    """A clock that only moves when told to. Call it to read the time."""

    def __init__(self, start=0.0):
        self.now = start

    def __call__(self):
        return self.now

    def advance(self, seconds):
        self.now += seconds
