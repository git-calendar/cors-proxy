# Git Calendar CORS Proxy
[![Go version](https://img.shields.io/github/go-mod/go-version/git-calendar/cors-proxy)](./go.mod)
[![Go](https://github.com/git-calendar/cors-proxy/actions/workflows/go.yaml/badge.svg)](https://github.com/git-calendar/cors-proxy/actions/workflows/go.yaml)
[![License](https://img.shields.io/github/license/git-calendar/cors-proxy)](./LICENSE.txt)

A small proxy that adds [CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS) headers to Git-over-HTTP responses.
It works around browser security restrictions when accessing third-party Git services such as GitHub, GitLab, and Codeberg, which we do not control.

> [!IMPORTANT]
> This proxy is not needed when `git-calendar/core` runs outside a browser.

## Build and run
### Bare metal
```sh
go run .
go build .
```

### Docker/Podman
```sh
docker build -t cors-proxy .
podman build -t cors-proxy .
```
```sh
docker run -d --rm \
  --name cors-proxy \
  -p 8080:8080 \
  cors-proxy

podman run -d --rm \
  --name cors-proxy \
  -p 8080:8080 \
  cors-proxy
```

### Environment variables
All variables are optional; the values below are the defaults:

```sh
HOST=0.0.0.0
PORT=8080
PRODUCTION=false
UPSTREAM_TIMEOUT=15s
MAX_RESPONSE_SIZE=1048576 # 1 MiB in bytes (1024^2)
ALLOWED_HOSTS=github.com,raw.githubusercontent.com,gitlab.com,codeberg.org
RATE_TOKENS=60
RATE_INTERVAL=1m
RATE_IP_SOURCE_HEADER="" # trusted client-IP header when behind a reverse proxy
ABUSE_URL=mailto:security@firu.dev
```

## Usage
A direct request to a Git smart HTTP endpoint may be blocked by the browser when the server does not return the required CORS headers:

```js
const target = "https://github.com/git/git.git/info/refs?service=git-upload-pack";
const response = await fetch(target);
```

Send the same request through the local proxy by placing the absolute target URL after the proxy address:

```js
const target = "https://github.com/git/git.git/info/refs?service=git-upload-pack";
const response = await fetch(`http://localhost:8080/${target}`);
const data = await response.arrayBuffer();
```

The proxy only accepts HTTP(S) paths ending in `.ics` or the Git smart HTTP endpoints `info/refs`, `git-upload-pack`, and `git-receive-pack`. The target host must also be listed in `ALLOWED_HOSTS`.

## Report abuse
Every deployment exposes `/.well-known/security.txt` with the contact configured by `ABUSE_URL`. The same contact is included in every upstream `User-Agent`. Operators should set it to their own monitored HTTPS page or `mailto:` address.

---
## I already use a reverse proxy
When hosting a [bare Git repository](https://git-scm.com/book/en/v2/Git-on-the-Server-Getting-Git-on-a-Server) on a [VPS](https://en.wikipedia.org/wiki/Virtual_private_server), you usually do not need this proxy because you control the server environment.

Instead, add the CORS headers with a [reverse proxy](https://en.wikipedia.org/wiki/Reverse_proxy) such as [Caddy](https://caddyserver.com/) or [Nginx](https://nginx.org/en/). The following Caddy configuration is based on [this excellent article](https://www.jamesatkins.com/posts/git-over-http-with-caddy/):
```caddyfile
your-repo-domain.com {
    # CORS setup (wildcards for origin, headers etc. often fail with credentials)
    header {
        Access-Control-Allow-Origin  "https://calendar-web-domain.com" # TODO
        Access-Control-Allow-Methods "GET, POST, OPTIONS" # git uses these HTTP methods
        Access-Control-Allow-Headers "Authorization, Content-Type, Git-Protocol" # git uses these headers
        Access-Control-Expose-Headers "Content-Length, Content-Range, Git-Protocol" # let client Wasm see those headers
        Access-Control-Allow-Credentials "true" # required for Basic Auth to work
    }

    # Handle preflight (OPTIONS) requests
    @options {
        method OPTIONS
    }
    respond @options 204 # No Content

    # Authentication
    basic_auth / {
        # generate password hash with 'caddy hash-password'
        your_vps_user_name $2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG
    }

    # Route to git-http-backend
    @git_cgi path_regexp "^.*/(HEAD|info/refs|objects/info/[^/]+|git-upload-pack|git-receive-pack)$"
    @git_static path_regexp "^.*/objects/([0-9a-f]{2}/[0-9a-f]{38}|pack/pack-[0-9a-f]{40}\.(pack|idx))$"
    vars git_dir /srv/git # or /home/git which contains repos like my_project.git/
    # make sure caddy and fcgiwrap have the right permissions to this directory
    # make sure selinux doesnt restrict the access
    
    handle @git_cgi {
        reverse_proxy unix//run/fcgiwrap.socket { # you will need fcgiwrap installed on your VPS
            transport fastcgi {
                env SCRIPT_FILENAME /usr/libexec/git-core/git-http-backend # depends on distro; find the executable by `find /usr -name "git-http-backend"`
                env GIT_HTTP_EXPORT_ALL 1
                env GIT_PROJECT_ROOT {vars.git_dir}
            }
        }
    }
    
    handle @git_static {
        file_server {
            root {vars.git_dir}
        }
    }
}
```
