"""Building and formatting the sales report."""

import csv
from collections import defaultdict


def load_rows(path):
    """Read the CSV at path into a list of (region, product, amount)."""
    with open(path, newline="") as f:
        return [(r["region"], r["product"], float(r["amount"])) for r in csv.DictReader(f)]


def build_report(rows):
    """Totals per region and per product, and the grand total."""
    by_region = defaultdict(float)
    by_product = defaultdict(float)
    for region, product, amount in rows:
        by_region[region] += amount
        by_product[product] += amount
    return {
        "regions": {k: round(v, 2) for k, v in sorted(by_region.items())},
        "products": {k: round(v, 2) for k, v in sorted(by_product.items())},
        "total": round(sum(a for _, _, a in rows), 2),
        "rows": len(rows),
    }


def format_text(report):
    """The report as aligned text lines."""
    lines = ["Sales report", "============", ""]
    lines.append("By region:")
    for name, total in report["regions"].items():
        lines.append(f"  {name:<10} {total:>10.2f}")
    lines.append("")
    lines.append("By product:")
    for name, total in report["products"].items():
        lines.append(f"  {name:<10} {total:>10.2f}")
    lines.append("")
    lines.append(f"Total: {report['total']:.2f} over {report['rows']} rows")
    return "\n".join(lines)
