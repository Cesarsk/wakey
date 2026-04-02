# Wakey

Wake your computer from your phone.

Wakey is a small Wake-on-LAN web server with a clean mobile UI, a simple JSON API, and a Docker-friendly setup for private LAN environments.

## Why Wakey

- one-tap wake button from any phone or browser
- editable target settings in the UI
- `POST /api/wake` for scripts, Shortcuts, and automations
- small Go service with no database and no frontend build step
- safe public repo layout with local secrets kept in `.env`

## What It Looks Like

Wakey is designed to feel simple on a phone:

- open the page
- tap `Wake Computer`
- optionally expand `Edit target settings`

The current target is shown on the main card so the page stays useful without feeling like an admin dashboard.

## How It Works

Wakey is a single Go binary that:

1. serves the web UI
2. accepts API requests
3. either sends the Wake-on-LAN magic packet directly or forwards the request to a tiny host-side helper

On macOS Docker runtimes, the most reliable setup is often:

1. run Wakey in Docker
2. run the included host helper on macOS
3. let the helper emit the WoL broadcast on the real LAN

## Requirements

- target machine connected over Ethernet
- Wake-on-LAN enabled in BIOS/UEFI and OS network settings
- correct target MAC address
- correct LAN broadcast address such as `192.168.1.255`
- a Docker environment for the web app
- on macOS, optionally the included host helper for reliable LAN packet delivery

## Quick Start

Copy the example config:

```sh
cp .env.example .env
```

Start the service:

```sh
docker compose up -d --build
```

If you are on macOS and direct container WoL is unreliable, start the included helper in a second terminal:

```sh
./tools/start_host_helper.sh
```

To install it as a per-user boot service on macOS:

```sh
./tools/install_launchd_helper.sh
```

To remove the boot service later:

```sh
./tools/uninstall_launchd_helper.sh
```

The installer copies the helper into `~/Library/Application Support/Wakey` and registers a `launchd` agent, which avoids macOS privacy issues with running login agents directly from `Documents`.

Open:

```text
http://localhost:8787
```

## Configuration

Available variables:

- `LISTEN_ADDR`: HTTP listen address, default `:8787`
- `WOL_TARGET_NAME`: label shown in the UI, default `My Computer`
- `WOL_MAC`: default target MAC address; optional if you prefer entering it in the UI
- `WOL_BROADCAST_ADDR`: default broadcast address, default `255.255.255.255`
- `WOL_PORT`: UDP port, default `9`; if your target does not respond, `7` is the next common fallback
- `WAKEY_HOST_HELPER_URL`: optional host helper endpoint such as `http://127.0.0.1:8788/wake`
- `WAKEY_BUTTON_LABEL`: button label shown in the UI
- `WAKEY_SUCCESS_MESSAGE`: success message returned by the UI and API
- `WAKEY_AUTH_TOKEN`: optional bearer token for `POST /api/wake`

If `WAKEY_HOST_HELPER_URL` is set, Wakey forwards wake requests to the helper instead of sending the packet from inside the container.
If `WAKEY_HOST_HELPER_TOKEN` is set, Wakey includes it in `X-Wakey-Helper-Token` and the helper rejects requests without the matching token.

## Web UI

The UI is intentionally compact.

- the wake action is front and center
- target settings live behind `Edit target settings`
- values are saved in browser `localStorage`
- your phone can remember the machine details without committing them to git

The browser can override:

- computer name
- MAC address
- broadcast address
- UDP port

## API

Wake the default configured machine:

```sh
curl -X POST http://localhost:8787/api/wake
```

Wake a specific machine with request overrides:

```sh
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"targetName":"Studio PC","macAddress":"AA:BB:CC:DD:EE:FF","broadcastAddress":"192.168.1.255","port":9}' \
  http://localhost:8787/api/wake
```

If token auth is enabled:

```sh
curl -X POST \
  -H "Authorization: Bearer $WAKEY_AUTH_TOKEN" \
  http://localhost:8787/api/wake
```

Success response:

```json
{"ok":true,"message":"Magic packet sent."}
```

## macOS Helper

The helper lives in `tools/host_wol_helper.rb` and only accepts one kind of request:

- `POST /wake`
- JSON body with `macAddress`, `broadcastAddress`, and `port`
- optional `X-Wakey-Helper-Token` header when token protection is enabled

Its only job is to build the magic packet and send it from the macOS host. It does not expose any broader shell access or arbitrary command execution.

By default the helper binds to `127.0.0.1`, which means it is not exposed on your LAN. In the current setup it is only reachable locally by Wakey running with host networking.

## Which Port Should I Use?

- start with UDP port `9`
- if wake does not work, try UDP port `7`
- on most LANs, the important part is the magic packet and broadcast delivery, not a special application protocol on the target machine

## How To Find Your Broadcast Address

If your machine IP is `192.168.1.42` and your subnet mask is `255.255.255.0`, your broadcast address is usually:

```text
192.168.1.255
```

Common examples:

- `192.168.1.42` with `255.255.255.0` -> `192.168.1.255`
- `10.0.0.23` with `255.255.255.0` -> `10.0.0.255`
- `192.168.0.15` with `255.255.255.0` -> `192.168.0.255`

If you are unsure, check your router or network settings and use the broadcast address for the target machine's subnet.

## Reverse Proxy Example

If you already run a private reverse proxy, you can point a hostname at Wakey like this:

```caddyfile
wakey.mini.example.com {
    reverse_proxy host.docker.internal:8787
}
```

## Security Notes

- Wakey is best used on a private LAN or behind a private reverse proxy
- do not commit real MAC addresses, tokens, or internal hostnames if you want the repo public
- if you expose the API beyond your private network, set `WAKEY_AUTH_TOKEN`
- Wake-on-LAN powers a machine on; it does not bypass login, encryption, or OS security

## Troubleshooting

- page does not load: make sure the container is running and `localhost:8787` responds
- wake request succeeds but the machine stays off: verify BIOS/UEFI WoL support and NIC power settings
- no wake over Wi-Fi: standard WoL is most reliable over wired Ethernet
- wrong broadcast address: try the subnet broadcast instead of the global broadcast
- shutdown wake fails but sleep wake works: this usually points to motherboard or NIC settings rather than Wakey itself
- on macOS, if direct container WoL does not work, use `WAKEY_HOST_HELPER_URL` with `./tools/start_host_helper.sh`
- if you want the helper at login/boot on macOS, install the included `launchd` agent with `./tools/install_launchd_helper.sh`

## Development

Useful commands:

```sh
docker compose up -d --build
docker compose logs -f
docker compose down
./tools/start_host_helper.sh
./tools/install_launchd_helper.sh
./tools/uninstall_launchd_helper.sh
```

## License

MIT
