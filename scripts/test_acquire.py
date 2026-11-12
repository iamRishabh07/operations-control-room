import subprocess
import unittest
from unittest.mock import patch
import acquire

class AcquisitionChecks(unittest.TestCase):
    def test_reject_invalid_integers(self):
        self.assertEqual(acquire.integer("123"), 123)
        for value in [-1, "-1", "1.5", 1.5, True, "9007199254740992"]:
            with self.subTest(value=value), self.assertRaises((ValueError, TypeError)):
                acquire.integer(value)

    def test_geometry_coordinates_preserve_lon_lat(self):
        geometry = [[[[ -87.8, 41.8], [-87.6, 42.0], [-87.8, 41.8]]]]
        self.assertEqual(list(acquire.coordinates(geometry)), [[-87.8, 41.8], [-87.6, 42.0], [-87.8, 41.8]])

    @patch("acquire.subprocess.run")
    def test_budget_and_http_failure_stop(self, run):
        fetch = acquire.Fetch()
        fetch.start -= 61
        with self.assertRaisesRegex(RuntimeError, "budget exhausted"):
            fetch("https://example.invalid")
        run.assert_not_called()
        fetch = acquire.Fetch()
        run.return_value = subprocess.CompletedProcess([], 22, b"", b"HTTP 503")
        with self.assertRaisesRegex(RuntimeError, "HTTP 503"):
            fetch("https://example.invalid")
        self.assertEqual(run.call_count, 1)
        self.assertIn("--max-filesize", run.call_args.args[0])
        self.assertIn("--max-time", run.call_args.args[0])

if __name__ == "__main__":
    unittest.main()
