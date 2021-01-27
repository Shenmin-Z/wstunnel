# wstunnel

## About

This project takes idea from [this](https://github.com/erebe/wstunnel), which takes idea from [this](https://www.npmjs.com/package/wstunnel).

For now it only supports outward tcp forwarding.

It's not real websocket, it just claims to be websocket when establishing connections so that proxy server would forward tcp packages for us.

## Usage

```
client:
-port-forward, -f: local and remote port pair, use ; as delimeter when there are multiple, e.g. 1111:1111;1112:1112
-server, -s      : address of websocket server
-proxy, -p       : address of proxy server
-password, -w    : optional, key to encryt data
-debug, -d       : show debug info
```

```
server:
-port, -p    : port on which server listens on, should be the same as the 'server' of client
-password, -w: optional, key to encryt data, should be the same client
-debug, -d   : show debug info
```

## Example

```bash
./wstunnel client \
  -f '6665:6789' \
  -s 'http://100.160.50.104:8080' \
  -p 'http://user:password@111.106.10.165:9090' \
  -w password \
  -d
```

```bash
./wstunnel server \
  -p 8080 \
  -w password \
  -d
```
