# Wakey

Wakey is a small Wake-on-LAN server with a phone-friendly web UI and a simple HTTP API. It is designed for private LAN use, so you can wake a desktop or home server from another device on your network.

## What it does

- sends a Wake-on-LAN magic packet to a target machine
- serves a compact mobile web page with a single wake button
- lets you edit the target machine settings in the browser
- exposes a `POST /api/wake` endpoint for Shortcuts, scripts, and automation
- runs cleanly in Docker on OrbStack using host networking

## How it works

Wakey is a single Go binary that:

1. serves the web UI
2. accepts API requests
3. sends a UDP broadcast magic packet to the configured MAC address

On macOS with OrbStack, `network_mode: host` lets the container use the host network directly, which is the simplest setup for Wake-on-LAN.

## Requirements

- target machine connected over Ethernet
- Wake-on-LAN enabled in BIOS/UEFI and OS network settings
- correct target MAC address
- correct LAN broadcast address, such as `192.168.1.255`
- OrbStack or another Docker environment that supports host networking for your use case

## Configuration

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Available variables:

- `LISTEN_ADDR`: HTTP listen address, default `:8787`
- `WOL_TARGET_NAME`: label shown in the UI, default `My Computer`
- `WOL_MAC`: default target MAC address; optional if you prefer entering it in the UI
- `WOL_BROADCAST_ADDR`: default broadcast address, default `255.255.255.255`
- `WOL_PORT`: UDP port, default `9`
- `WAKEY_BUTTON_LABEL`: button label shown in the UI
- `WAKEY_SUCCESS_MESSAGE`: success message returned by the UI and API
- `WAKEY_AUTH_TOKEN`: optional bearer token for `POST /api/wake`

## Web UI behavior

The web interface keeps the wake action front and center, with editable settings behind an `Edit target settings` panel.

The browser can override the default target values for:

- computer name
- MAC address
- broadcast address
- UDP port

Those values are stored in browser `localStorage`, so your phone can remember them without committing any private machine details to the repository.

## Run locally

```sh
go run .
```

The app listens on `http://localhost:8787` by default.

## Run with Docker Compose

```sh
docker compose up -d --build
```

Compose uses host networking so the container can send WoL traffic onto your LAN.

## API

Wake the default configured machine:

```sh
curl -X POST http://localhost:8787/api/wake
```

Wake a specific machine by sending the target details in JSON:

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

Successful responses look like:

```json
{"ok":true,"message":"Magic packet sent."}
```

## Private reverse proxy example

If you already use a private Caddy ingress, you can reverse proxy Wakey like this:

```caddyfile
wakey.mini.example.com {
    reverse_proxy host.docker.internal:8787
}
```

## Security notes

- Wakey is best used on a private LAN or behind a private reverse proxy such as Tailscale + Caddy
- do not commit real MAC addresses, tokens, or internal DNS names unless you want them public
- if you expose the API beyond your private network, set `WAKEY_AUTH_TOKEN`
- Wake-on-LAN only powers a machine on; it does not bypass login or disk encryption

## Troubleshooting

- page does not load: make sure the container is running and `localhost:8787` responds
- wake request succeeds but machine stays off: verify BIOS/UEFI WoL support and Ethernet link lights when powered off
- no wake over Wi-Fi: standard WoL is most reliable over wired Ethernet
- wrong broadcast address: try your subnet broadcast such as `192.168.1.255` instead of the global broadcast

## Development

Useful commands:

```sh
docker compose up -d --build
docker compose logs -f
docker compose down
```
