"""Command line entry point: python -m reportgen.cli FILE."""

import argparse
import sys

from reportgen.report import build_report, format_text, load_rows


def main(argv=None):
    parser = argparse.ArgumentParser(prog="reportgen", description="Summarize a sales CSV.")
    parser.add_argument("file", help="CSV with region,product,amount columns")
    args = parser.parse_args(argv)
    report = build_report(load_rows(args.file))
    print(format_text(report))
    return 0


if __name__ == "__main__":
    sys.exit(main())
