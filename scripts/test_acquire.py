import contextlib
import datetime as dt
import io
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import acquire

SQUARE = [[[[-87.7, 41.8], [-87.6, 41.8], [-87.6, 41.9], [-87.7, 41.9], [-87.7, 41.8]]]]
AREA_ROWS = [
    {"area_num_1": str(i), "community": f"AREA {i}", "the_geom": {"type": "MultiPolygon", "coordinates": SQUARE}}
    for i in range(1, 78)
]
HOUR_ROWS = [{"area_id": "8", "trips": "10"}, {"trips": "2"}]  # 10 in area 8, 2 with no area


class AcquisitionChecks(unittest.TestCase):
    def test_reject_invalid_integers(self):
        self.assertEqual(acquire.integer("123"), 123)
        for value in [-1, "-1", "1.5", 1.5, True, "9007199254740992"]:
            with self.subTest(value=value), self.assertRaises((ValueError, TypeError)):
                acquire.integer(value)

    def test_geometry_coordinates_preserve_lon_lat(self):
        self.assertEqual(
            list(acquire.coordinates(SQUARE))[:2], [[-87.7, 41.8], [-87.6, 41.8]]
        )

    def test_windows_partition_the_period_half_open(self):
        wins = acquire.windows(dt.date(2022, 9, 12), 1)
        self.assertEqual(len(wins), 7 * 24)
        self.assertEqual(wins[0][2], "2022-09-12T00:00:00")
        self.assertEqual(wins[-1][3], "2022-09-19T00:00:00")
        for current, following in zip(wins, wins[1:]):
            self.assertEqual(current[3], following[2])
        # An inclusive upper bound would count every HH:00 trip in two hours.
        self.assertIn(">= '{start}'", acquire.HOUR_QUERY)
        self.assertIn("< '{end}'", acquire.HOUR_QUERY)
        self.assertNotIn("between", acquire.HOUR_QUERY.lower())

    def test_parse_hour_counts_the_no_area_group_as_excluded(self):
        self.assertEqual(acquire.parse_hour(HOUR_ROWS), ({8: 10}, 2))

    def test_parse_hour_rejects_unexpected_groups(self):
        bad = {
            "area out of range": [{"area_id": "78", "trips": "1"}],
            "duplicate area": [{"area_id": "8", "trips": "1"}, {"area_id": "8", "trips": "2"}],
            "duplicate no-area group": [{"trips": "1"}, {"trips": "2"}],
            "fractional count": [{"area_id": "8", "trips": "1.5"}],
            "not a list": {"error": True},
        }
        for name, rows in bad.items():
            with self.subTest(name), self.assertRaises((ValueError, KeyError)):
                acquire.parse_hour(rows)

    @patch("acquire.subprocess.run")
    def test_fetch_retries_then_reports_the_source_unavailable(self, run):
        run.return_value = subprocess.CompletedProcess([], 22, b"", b"HTTP 503")
        with self.assertRaisesRegex(acquire.SourceUnavailable, "HTTP 503"):
            acquire.fetch("https://example.invalid", {}, tries=3, pause=lambda _: None)
        self.assertEqual(run.call_count, 3)
        self.assertIn("--max-time", run.call_args.args[0])


class PublishGate(unittest.TestCase):
    """main() must publish only a complete, reconciled extract."""

    def run_main(self, hourly, count):
        def fake_fetch(url, params, **_):
            if url == acquire.BOUNDARIES:
                return AREA_ROWS
            if "GROUP BY" in params["$query"]:
                return hourly()
            return [{"trips": str(count)}]

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with patch.multiple(
                acquire,
                fetch=fake_fetch,
                CACHE=root / "cache",
                SEED=root / "seed.json",
                MANIFEST=root / "manifest.json",
                WEEKS=1,
                MAX_FAILED=0,
            ), contextlib.redirect_stdout(io.StringIO()):
                code = acquire.main()
            seed = json.loads((root / "seed.json").read_text()) if (root / "seed.json").exists() else None
        return code, seed

    def test_complete_and_reconciled_extract_is_published(self):
        code, seed = self.run_main(lambda: HOUR_ROWS, count=24)  # two hours x (10 + 2)
        self.assertEqual(code, 0)
        self.assertEqual(len(seed["hours"]), 7 * 24)
        self.assertEqual(len(seed["hours"][0]["trips"]), 77)
        self.assertEqual(seed["hours"][0]["trips"][7], 10)  # area 8 sits at index 7
        self.assertEqual(seed["history"]["trips_included"], 7 * 24 * 10)
        self.assertEqual(seed["history"]["trips_excluded_invalid_area"], 7 * 24 * 2)
        self.assertEqual(seed["history"]["period_end"], "2022-09-18")

    def test_unavailable_source_publishes_nothing(self):
        def down():
            raise acquire.SourceUnavailable("HTTP 503")

        code, seed = self.run_main(down, count=24)
        self.assertEqual(code, 1)
        self.assertIsNone(seed)

    def test_empty_response_counts_as_a_failed_window(self):
        code, seed = self.run_main(lambda: [], count=24)
        self.assertEqual(code, 1)
        self.assertIsNone(seed)

    def test_reconciliation_mismatch_publishes_nothing(self):
        code, seed = self.run_main(lambda: HOUR_ROWS, count=30)
        self.assertEqual(code, 1)
        self.assertIsNone(seed)


if __name__ == "__main__":
    unittest.main()
