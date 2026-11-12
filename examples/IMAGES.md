# Your own images, built with Podman or Docker

vmx runs OCI images, the kind Podman and Docker build and every registry
holds, each in a Firecracker microVM of its own. vmx itself builds no
image: the appliance installs nothing, so the image is built on any other
Linux, Mac or Windows machine, and reaches the vmx host one of two ways.

## 1. Build it

A web service in Go, as an example (`scene/` holds two more):

```sh
mkdir myapp && cd myapp
cat > main.go <<'EOF'
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	host, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from %s, mode %s\n", host, os.Getenv("MODE"))
	})
	log.Printf("myapp on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
EOF
cat > Containerfile <<'EOF'
FROM docker.io/library/golang:1.24 AS build
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 go build -o /myapp main.go

FROM scratch
COPY --from=build /myapp /myapp
EXPOSE 8080
ENTRYPOINT ["/myapp"]
EOF
podman build -t myapp:1 .          # or: docker build -t myapp:1 .
```

Any image works: one made `FROM alpine`, `FROM debian`, `FROM scratch`
with one static binary, or an image someone else publishes.

## 2a. From a registry

Push it where the vmx host can reach it, and pull it there:

```sh
podman push myapp:1 ghcr.io/you/myapp:1       # on the build machine
vmx container pull ghcr.io/you/myapp:1        # on the vmx host
```

`IMAGE=ghcr.io/you/myapp:1 PORT=8080 COUNT=3 ./my-image.sh` does the pull
and runs three copies.

## 2b. Carried as a file, for a host that reaches no registry

```sh
# On the build machine: the archive, and the digest that names the image.
podman save --format oci-archive -o myapp.tar myapp:1      # or: docker save -o myapp.tar myapp:1 (Docker 25 and later)
tar -xOf myapp.tar index.json | jq -r '.manifests[0].digest'
scp myapp.tar root@vmx-host:/home/

# On the vmx host: the digest is the one the build machine printed. Nothing
# in an archive vouches for itself, so vmx checks every blob against it.
vmx container import /home/myapp.tar myapp:1 --digest sha256:THE-DIGEST-PRINTED-ABOVE
```

`ARCHIVE=/home/myapp.tar DIGEST=sha256:... IMAGE=myapp:1 PORT=8080 COUNT=3 ./my-image.sh`
does the import and runs three copies.

## 3. Run it

```sh
vmx container run myapp:1 app-01 --port 9001:8080 --env MODE=demo
curl http://vmx-host:9001/        # from your network: hello from app-01, mode demo
vmx container logs app-01
```

What `docker run` takes, `vmx container run` takes too: `--env`,
`--port HOST:CONTAINER`, `--volume NAME:/PATH`, `--user`, `--workdir`,
`--entrypoint`, and a command after `--`. Each container has 1 CPU and
256 MiB unless `--cpus` and `--memory` say otherwise.
