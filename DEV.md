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


## Full list of environment variables 

The following environment variables are used to configure the `CBOM-Repository`:

|   Environment Variable   |  Required  | Default Value | Explanation |
| :----------------------: | :--------: | :-------------: | :-------------: |
| APP_LOG_LEVEL            | Yes | "INFO" | logger level, possible values: \[ DEBUG, INFO, WARN, ERROR \] |
| APP_HTTP_PORT            | Yes | 8080 | HTTP server port |
| APP_S3_ACCESS_KEY        | Yes | - | s3-compatible store access key |
| APP_S3_SECRET_KEY        | Yes | - | s3-compatible store secret key |
| APP_S3_REGION            | Yes | - | s3-compatible store Region |
| APP_S3_ENDPOINT          | No | - | s3-compatible store endpoint, leave empty for aws roles or default aws env. variables to take precedence |
| APP_S3_BUCKET            | Yes | - | bucket name |
| APP_S3_USE_PATH_STYLE    | Yes | true | Use s3 path style |


