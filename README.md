# wstunnel

## About

`wstunnel` allows you to use **tcp** in restricted environment where the only access to outside world is an **http** proxy.

This project takes idea from [this](https://github.com/erebe/wstunnel), which takes idea from [this](https://www.npmjs.com/package/wstunnel).

All code is in a [single file](https://github.com/Shenmin-Z/wstunnel/blob/main/wstunnel.go) (less than 500 lines) and has *no dependency*.

## Usage

`wstunnel` works like this:
```
source process <=> client <=> proxy <=> server <=> destination process
```

`client` runs in the restrcited environment:
```
client:
-port-forward: local and remote port pair, use ; as delimeter when there are multiple, e.g. 8888:8888;8889:9001
-server      : address of wstunnel server
-proxy       : address of proxy server
-password    : optional, key to encryt data
-debug       : show debug info
```

`server` runs in the outside world:
```
server:
-port    : port on which server listens on
-password: optional, key to encryt data
-debug   : show debug info
```

## Example: ssh into outside server

Destination server ip is *100.100.100.100* and it listens on port 22 for ssh connections.

Http proxy is running on *111.111.111.111:9090* which uses basic http authentication.

```bash
./wstunnel client \
  -port-forward '6665:22' \
  -server 'http://100.100.100.100:8080' \
  -proxy 'http://user:password@111.111.111.111:9090' \
  -password password \
  -debug
```

```bash
./wstunnel server \
  -port 8080 \
  -password password \
  -debug
```

This way, local port 6665 is tunnelled to remote port 22.

Now ssh can be run like `ssh -p 6665 user@127.0.0.1`.

The `8080` part may need to be changed to something like `443` if http proxy doesn't allow random port.

## Install

[Binary](https://github.com/Shenmin-Z/wstunnel/releases) or build from source `go build wstunnel.go`.
