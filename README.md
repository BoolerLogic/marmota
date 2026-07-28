# Marmota

<img src="./images/marmota_icon.png" width="350" alt="Marmota logo" />

Marmota is an experimental, cross-platform desktop proxy for inspecting, filtering, and replaying HTTP and HTTPS traffic. It combines a Go proxy backend with a Wails and Svelte interface, and is intended for developer debugging, research, and authorized security work.

Marmota is not intended to replace full security-testing platforms such as Burp Suite or OWASP ZAP. Its focus is a lightweight desktop workflow for traffic inspection, filtering, request export, and replay.

> [!WARNING]
> Only inspect systems, devices, and networks for which you have explicit permission. Intercepted traffic can contain credentials, session tokens, personal data, and other sensitive information.

## Project Status

Marmota is under active development. Its behavior, interface, and file formats may change between releases. Bug reports and feedback about the proxy, UI, and documentation are welcome.

## Download and Distribution

Download the latest release for Windows, macOS, or Linux from the [Releases page](https://github.com/BoolerLogic/Marmota/releases).

### Windows and macOS

Windows and macOS releases are portable applications, not installers. Moving or duplicating the application does not create a separate Marmota profile: every copy run by the same operating-system user uses the same configuration directory, CA, and proxy settings.

### Linux

Linux builds are distributed as installable packages:

- Use the `webkit4.1` build on Ubuntu 24.04+, Mint 22+, Debian 13, Fedora 40+, and rolling distributions such as Arch Linux.
- Use the `webkit4.0` build on Ubuntu 22.04 or older, Mint 21, Debian 12 or older, and Fedora 39 or older.
- Debian and Ubuntu users should choose the `.deb` package.
- Fedora and Red Hat users should choose the `.rpm` package.

## Quick Start

1. Launch Marmota and keep the default listener at `127.0.0.1:8080`.
2. Select **Start Proxy**.
3. Configure your browser or test client to use `127.0.0.1` on port `8080` as both its HTTP and HTTPS proxy.
4. To inspect HTTPS, install and trust Marmota's `ca.crt` certificate in the browser or operating-system trust store. The configuration screen shows its folder and can open it directly.
5. Visit an HTTPS website. Its requests and responses should appear in **Traffic Inspector**.
6. When finished, stop Marmota and disable the proxy in your browser or client.

If you no longer intend to use Marmota, remove its CA from every trust store where you installed it. Deleting Marmota's local certificate file alone does not remove an already trusted certificate.

## Trust, Storage, and Cleanup

### Configuration Directory

Marmota stores its generated CA and persistent proxy configuration in the current user's operating-system configuration directory:

| Operating system | Default directory |
| --- | --- |
| Windows | `%APPDATA%\marmota` |
| macOS | `~/Library/Application Support/marmota` |
| Linux | `$XDG_CONFIG_HOME/marmota`, or `~/.config/marmota` when `XDG_CONFIG_HOME` is unset |

The configuration screen displays the exact directory in use and provides a shortcut to open it. It normally contains:

- `ca.crt`: the CA certificate that can be installed in a client trust store.
- `ca.key`: the CA private key. Keep this file private.
- `config.json`: the most recently started marmota configuration.

The CA is generated when it does not already exist and is then reused. Consequently, multiple portable Windows or macOS copies run by the same user share the same CA. Resetting the CA creates a new identity; clients must stop trusting the old certificate and trust the new one if HTTPS interception is still required.

To remove Marmota completely:

1. Stop the proxy and close the application.
2. Disable Marmota as a proxy in every configured browser, device, or operating system.
3. Remove Marmota's CA from every browser and operating-system trust store where it was installed.
4. Delete the Marmota configuration directory if you no longer need its settings or CA files.

If the application is launched again after those files are deleted, it creates a new CA and configuration directory as needed.

### Data Retention

Captured HTTP history and **Saved Requests** are kept in memory for the current application session and are cleared when Marmota exits. Captured request and response bodies are not automatically written to disk.

Marmota does persist the listener, TLS, and optional outbound SOCKS5 settings in `config.json` whenever **Start Proxy** is selected. The file is loaded once at application startup. SOCKS5 usernames and passwords are stored as plain text in this local file, so protect the configuration directory accordingly.

Traffic Inspector filters and sorting remain available while switching between sections during the current application session, but are reset when Marmota exits. Marmota does not automatically redact secrets from captured traffic.

## Core Features

### Traffic Interception and Inspection

Marmota records HTTP and HTTPS requests routed through its listener and presents their headers, bodies, URLs, status, timing, and connection metadata in a desktop Traffic Inspector.

![Traffic Inspector](./images/inspector.png)

### Backend Filtering

Filtering runs in the Go backend rather than over rendered table rows in the UI. Each filter opens in its own tab, preserving the main history and other filtered views. Conditions can target requests, responses, headers, bodies, method, host, port, scheme, or path and can be combined with `AND` and `OR`.

![Filtering interface](./images/filtering.png)

### Search and Highlighting

Search within a selected entry globally or restrict matches to the request head, request body, response head, or response body. Matching text is highlighted and can be navigated from the search controls.

![Selected request and response](./images/select.png)

### Payload Decoding and Formatting

Marmota recognizes common formats including JSON, HTML, URL-encoded forms, and multipart form data, and provides syntax highlighting and pretty printing where applicable.

Captured bodies can be decoded from `gzip`, zlib or raw `deflate`, Brotli (`br`), Zstandard (`zstd`), and stacked `Content-Encoding` values. A response with an unsupported encoding remains available as raw data and receives an orange warning in Traffic Inspector.

HTML responses can be opened in an isolated visual preview. Scripts, forms, navigation, popups, and external network loading are blocked so the captured markup can be viewed without acting as a normal browsing context.

### Snippet Export

Export a captured request as a URL, cURL command, JavaScript `fetch` or `axios` code, Python `requests` or `httpx` code, or standalone JavaScript/Python header structures.

![Snippet export](./images/export.png)

### Request Repeater

Send a captured request to Repeater, edit its headers or body, validate its basic HTTP syntax, and replay it against the destination server. Marmota does not currently pause live traffic for interactive editing before forwarding.

### Session-Only Saved Requests

Bookmark important traffic in **Saved Requests** for the current application session. This list is intentionally not persisted and is cleared when Marmota exits.

## Network and Security Configuration

![Proxy configuration](./images/configuration.png)

### HTTPS Interception

Marmota generates a local CA and uses it to issue certificates for HTTPS destinations. A browser or client must trust `ca.crt` before it will accept those intercepted connections. The CA private key remains in the Marmota configuration directory and should never be shared.

### Upstream TLS Verification

Upstream certificate verification is enabled by default. Marmota can disable it for self-signed, expired, or otherwise untrusted development certificates, but doing so prevents Marmota from authenticating destination servers and makes upstream connections vulnerable to interception. Enable this option only for a controlled environment where it is genuinely required.

### Listener Binding

The listener can bind to:

- `localhost` (`127.0.0.1`) for clients on the same computer.
- A specific network-interface address.
- All interfaces (`0.0.0.0`) for clients that can reach the computer over the network.

> [!CAUTION]
> Marmota does not authenticate inbound proxy clients. Bind to `0.0.0.0` only on a trusted LAN, restrict access with a firewall, and never expose the listener directly to an untrusted network or the Internet.

### HTTP Protocol Support

Marmota forwards and inspects cleartext HTTP/1.1 traffic. It also handles HTTP/1.1 and HTTP/2 traffic negotiated inside intercepted TLS connections. Cleartext HTTP/2 (`h2c`) is not currently supported; Marmota responds with `505 HTTP Version Not Supported` and tells the client to use HTTP/1.1 or HTTPS.

### Optional Outbound SOCKS5 Proxy

Marmota can route its outbound destination connections through a SOCKS5 proxy instead of connecting directly from the local machine. Host and port are required; username and password are optional and are used when the SOCKS5 server requests authentication.

This creates the following route:

```text
Browser or client -> Marmota -> SOCKS5 proxy -> Destination
```

Inbound HTTPS can still be intercepted by Marmota while the resulting outbound connections use the configured SOCKS5 route. If Marmota's listener is accessible from other devices, those devices could use your configured SOCKS5 proxy and consume its traffic or quota. Restrict access with a firewall.

## Current Limitations

- HTTP/3 and QUIC are not supported.
- Cleartext HTTP/2 (`h2c`) is not supported.
- Applications that enforce certificate pinning may reject Marmota's generated certificates.
- Marmota's listener has no username or password protection. Any client that can reach it can use the proxy.
- WebSocket messages are not available for individual frame-by-frame inspection.
- Captured history and Saved Requests are not persisted between sessions.
- Marmota does not automatically hide sensitive captured data such as passwords, cookies, API keys, or authorization tokens.
- Body capture is limited to 500 KiB per request or response for inspection; large or binary payloads may therefore be incomplete in the UI.
- Live requests cannot be paused and edited before forwarding; use Repeater to modify and resend a captured request.

## Build from Source

### Prerequisites

- [Go](https://go.dev/dl/) 1.24 or newer.
- [Node.js](https://nodejs.org/) and npm.
- [Wails CLI and platform dependencies](https://wails.io/docs/gettingstarted/installation).

### Build Instructions

1. Clone the repository:
   ```bash
   git clone https://github.com/BoolerLogic/marmota.git
   cd marmota
   ```

2. Build the application for your current OS:
   ```bash
   wails build
   ```
   *The compiled binary will be located in the `build/bin/` directory.*

### Development Mode
To run Marmota in development mode with hot-reloading:
```bash
wails dev
```

## Technology

- Backend: Go
- Desktop framework: Wails
- Frontend: Svelte

Frontend development has been assisted by OpenAI Codex.

## License

Marmota is licensed under the MIT License. See [LICENSE](LICENSE) for details.
