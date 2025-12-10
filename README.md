# CBOM-Repository

The CBOM Repository service acts as an object storage wrapper built on top of an S3-compatible backend.
It provides a convenient REST API for uploading, retrieving, and searching Cryptographic Bills of Materials (CBOM) documents.

## Installation

Use the provided Helm chart to deploy the service into your Kubernetes cluster.
Please refer to the [Helm chart README](./deploy/charts/cbom-repository/README.md) for detailed installation instructions and configuration options.

## Status Work In Progress

This project is currently under active development.

* You can explore the current REST API design at [OpenAPI Spec](./api/openapi.yaml).
* To run the service locally or see development notes, please continue to the development guide [here](./DEV.md).

## API Endpoints

A summary of the available endpoints and methods are below. For the complete specification please see [OpenAPI Spec](./api/openapi.yaml).

| Path            | HTTP Method | Required Params                                               | Optional Params           | Description                                                                                                    |
|:----------------|:------------|:--------------------------------------------------------------|:--------------------------|:---------------------------------------------------------------------------------------------------------------|
| `/v1/bom`       | `POST`      | Contents of BOM in request body and `Content-Type` header set |                           | Uploads the supplied BOM to the repository                                                                     |
| `/v1/bom`       | `GET`       | query parameter `after`                                       |                           | Retrieves a list of BOM serial numbers and versions that were created later that `after` timestamp             |
| `/v1/bom/{urn}` | `GET`       |                                                               | query parameter `version` | If optional query parameter `version` is not supplied, retrieves the latest version of the BOM from repository |

Let's see each endpoint in greater detail.

### POST /v1/bom (Upload)

The upload operation requires a valid `Content-Type` header. At this time, only JSON format using CycloneDX Schema version 1.6 is supported.
This means the `Content-Type` header must be set to: 
```
application/vnd.cyclonedx+json
```

Optionally, you may specify an explicit version, for example:
```
application/vnd.cyclonedx+json; Version=1.6
```

If a version is provided, the handler will validate the uploaded BOM document against the corresponding CycloneDX schema specification.
If no version is supplied, the handler will attempt to decode the BOM and automatically determine the correct schema version to validate against.

Support for additional formats, including the upcoming CycloneDX 1.7 specification, is planned to be added shortly.

#### Upload behavior

When processing uploaded BOMs, the system recognizes several use cases:

* **BOM includes both a serial number and a version.**
  The BOM is stored exactly as provided. Subsequent attempts to upload the same serial number and version will result in a 409 Conflict response.
* **BOM includes a serial number but no version.**
  The storage layer is checked for existing BOMs with the same serial number: 
  * If matching entries are found, the uploaded BOM is assigned the next version number (latest version + 1).
  * If none exist, the uploaded BOM is stored as Version 1.
* **BOM includes neither a serial number nor a version.**
  A new URN is generated automatically. Two BOMs are stored:
  1. The original, potentially cryptographically signed, stored under the new URN with version original.
  2. A normalized version, where a serial number and version have been assigned, stored under the same URN with version 1.

Upon successful upload, the endpoint returns basic cryptographic statistics about the provided BOM.

This feature is still a work in progress, and both the format and the details reported may evolve over time.

### GET /v1/bom (Search)

The search operation requires a single query parameter: `after`, whose value must be a Unix timestamp.
The endpoint responds with a list of URNs along with all versions created after the specified timestamp.
This allows clients to efficiently discover updates without scanning the entire BOM collection.

### GET /v1/bom/{urn} (Get by URN)

The get operation retrieves the latest version of a BOM—i.e., the entry with the highest version number—based on the {urn} supplied in the URL path.

The value of {urn} must conform to RFC 4122, meaning it follows the format:
```
urn:<NID>:<NSS>
```
Where:
* `<NID>` — Namespace Identifier, which must be exactly uuid for RFC 4122.
* `<NSS>` — Namespace-Specific String, which must be a valid UUID.

To retrieve a specific version instead of the latest, you may provide the optional query parameter:
```
?version=<number>
```
