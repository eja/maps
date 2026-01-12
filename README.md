# Maps

A lightweight, high-performance map tile server written in Go. It is designed to serve geospatial data directly from **MBTiles** archives and **PMTiles** containers with minimal configuration. It features a built-in web viewer, basic authentication, and supports embedded usage.

## Features

*   **Format Support:**
    *   **MBTiles:** Server-side rendering of tiles from SQLite-based archives.
    *   **PMTiles:** Optimized streaming of static archives with HTTP Range request support.
*   **Web Interface:** Includes a built-in map viewer powered by **MapLibre GL JS**, allowing for instant preview and inspection of vector tiles.
*   **Zero External Dependencies:** Uses a pure Go SQLite implementation (`modernc.org/sqlite`), removing the need for CGO or external system libraries.
*   **Security:** Built-in HTTP Basic Authentication and secure path traversal protection.
*   **Flexible Deployment:** Can serve a single file or an entire directory of map archives.
*   **Metadata API:** Exposes internal metadata from MBTiles files via JSON endpoints.

## Installation

### Build from Source

You can build the project using the provided `Makefile`.

```bash
# Clone the repository
git clone https://github.com/eja/maps.git
cd maps

# Build the binary
make maps

# (Optional) Build a static binary
make static
```

The resulting binary will be placed in the `build/` directory.

## Usage

The server can be started via the command line. It requires a target file or directory path.

```bash
./build/maps [options]
```

### Configuration Options

| Flag          | Default         | Description                                      |
|---------------|-----------------|--------------------------------------------------|
| `-file-path`  | `.`             | Path to a specific map file or a directory containing maps. |
| `-web-host`   | `localhost`     | The interface/host address to bind to.           |
| `-web-port`   | `35248`         | The TCP port to listen on.                       |
| `-web-path`   | `/maps/`        | The URL prefix for the application.              |
| `-web-auth`   | *(empty)*       | HTTP Basic Auth credentials in `user:password` format. |
| `-log`        | `false`         | Enable logging to stdout.                        |
| `-log-file`   | *(empty)*       | Path to a specific log file (implies `-log`).    |

### Examples

**Serve a directory of maps:**
```bash
./build/maps -file-path /var/data/maps
```
Access the server at `http://localhost:35248/maps/`.

**Serve a specific MBTiles file with Authentication:**
```bash
./build/maps -file-path ./europe.mbtiles -web-auth "admin:secret123" -log
```

**Run on a public interface:**
```bash
./build/maps -web-host 0.0.0.0 -web-port 8080
```

## API & Endpoints

Assuming the default prefix `/maps/`:

*   **`GET /maps/map/{filename}/`**
    *   Returns the HTML viewer for the specified map file.
*   **`GET /maps/map/{filename}/{z}/{x}/{y}`**
    *   Returns the raw tile data (Protobuf/Gzip) for the given coordinates.
    *   Automatically handles XYZ to TMS coordinate conversion for MBTiles.
*   **`GET /maps/map/{filename}/metadata.json`**
    *   Returns the metadata table of the MBTiles file as JSON.
*   **`GET /maps/map/{filename}.pmtiles`**
    *   Serves the raw file with support for HTTP `Range` headers (compatible with MapLibre PMTiles protocol).

