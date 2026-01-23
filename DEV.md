# Development guide

## How to run `CBOM-Repository` locally

### Prerequisites 

Since `CBOM-Repository` is a wrapper over an s3-compatible object storage, we need to make sure that the object storage is up and running before we start the service itself. Please run `make docker-compose-up` to start a MinIO docker container and create a bucket in it.

Once you are finished, please run `make docker-compose-down` to clean up.

### Configuration

In the root folder of this repository, please create a file, for example `.envrc.dev`, and paste the following text into it:
```bash
#!/bin/bash

APP_S3_ACCESS_KEY="minioadmin"
APP_S3_SECRET_KEY="minioadmin"
APP_S3_REGION="eu-west-1"
APP_S3_ENDPOINT="http://localhost:9000"
APP_S3_BUCKET="czert"
```

then make an executable bash script:
```bash
#!/bin/bash

set -o allexport
source .envrc.dev
set +o allexport

make build
./artifacts/svc
```
and run it.
