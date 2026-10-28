import io
import os
import unittest
from contextlib import redirect_stdout

from reportgen import cli
from reportgen.report import build_report, format_text

HERE = os.path.dirname(__file__)
SALES = os.path.join(HERE, "..", "sales.csv")


class ReportTest(unittest.TestCase):
    def test_build_report(self):
        r = build_report([("n", "w", 1.0), ("s", "w", 2.5), ("n", "g", 3.0)])
        self.assertEqual(r["regions"], {"n": 4.0, "s": 2.5})
        self.assertEqual(r["products"], {"g": 3.0, "w": 3.5})
        self.assertEqual(r["total"], 6.5)
        self.assertEqual(r["rows"], 3)

    def test_text(self):
        text = format_text(build_report([("n", "w", 1.0)]))
        self.assertIn("Total: 1.00 over 1 rows", text)

    def test_cli_text(self):
        out = io.StringIO()
        with redirect_stdout(out):
            self.assertEqual(cli.main([SALES]), 0)
        self.assertTrue(out.getvalue().startswith("Sales report"))


if __name__ == "__main__":
    unittest.main()
