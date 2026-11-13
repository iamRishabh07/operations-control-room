#!/usr/bin/env python3
"""Local stand-in for the Open-Meteo forecast endpoint, for demos only.

Answers the request shape the Go weather adapter sends. The first requested
location gets --precip mm in the current 15 minutes; the others stay dry.
Start the server with `-weather-url http://127.0.0.1:9099/` (see `make demo`);
the API and the UI then label every reading as simulated.
"""
import argparse
import json
import socketserver
import time
from http.server import BaseHTTPRequestHandler
from urllib.parse import parse_qs, urlparse

PORT = 9099


class Server(socketserver.TCPServer):
    # Plain TCPServer: http.server.HTTPServer resolves its own hostname on
    # bind, which can stall for seconds before the socket starts listening.
    allow_reuse_address = True


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--precip", type=float, default=0.6, help="mm per 15 minutes at the first location")
    precip = parser.parse_args().precip

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            latitudes = parse_qs(urlparse(self.path).query).get("latitude", [""])[0]
            stamp = int(time.time()) // 900 * 900  # start of the running 15-minute interval
            readings = [
                {
                    "current_units": {"precipitation": "mm"},
                    "current": {"time": stamp, "interval": 900, "precipitation": precip if i == 0 else 0.0},
                }
                for i in range(latitudes.count(",") + 1)
            ]
            body = json.dumps(readings if len(readings) > 1 else readings[0]).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    print(f"simulated weather on http://127.0.0.1:{PORT}/ ({precip} mm at the first location)", flush=True)
    Server(("127.0.0.1", PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
