package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/CZERTAINLY/CBOM-Repository/internal/service"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/stretchr/testify/require"
)

func TestStats1(t *testing.T) {
	jsonBom := `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.6",
  "serialNumber": "urn:uuid:e8c355aa-2142-4084-a8c7-6d42c8610ba2",
  "version": 1,
  "metadata": {
    "timestamp": "2024-01-09T12:00:00Z",
    "component": {
      "type": "application",
      "name": "my application",
      "version": "1.0"
    }
  },
  "components": [
    {
      "name": "RSA-2048",
      "type": "cryptographic-asset",
      "bom-ref": "crypto/key/rsa-2048@1.2.840.113549.1.1.1",
      "cryptoProperties": {
        "assetType": "related-crypto-material",
        "relatedCryptoMaterialProperties": {
          "type": "public-key",
          "id": "2e9ef09e-dfac-4526-96b4-d02f31af1b22",
          "state": "active",
          "size": 2048,
          "algorithmRef": "crypto/algorithm/rsa-2048@1.2.840.113549.1.1.1",
          "securedBy": {
            "mechanism": "Software",
            "algorithmRef": "crypto/algorithm/aes-128-gcm@2.16.840.1.101.3.4.1.6"
          },
          "creationDate": "2016-11-21T08:00:00Z",
          "activationDate": "2016-11-21T08:20:00Z"
        },
        "oid": "1.2.840.113549.1.1.1"
      }
    },
    {
      "name": "RSA-2048",
      "type": "cryptographic-asset",
      "bom-ref": "crypto/algorithm/rsa-2048@1.2.840.113549.1.1.1",
      "cryptoProperties": {
        "assetType": "algorithm",
        "algorithmProperties": {
          "parameterSetIdentifier": "2048",
          "executionEnvironment": "software-plain-ram",
          "implementationPlatform": "x86_64",
          "cryptoFunctions": [ "encapsulate", "decapsulate" ]
        },
        "oid": "1.2.840.113549.1.1.1"
      }
    },
    {
      "name": "RSA-2048",
      "type": "cryptographic-asset",
      "bom-ref": "crypto/algorithm/rsa-2048@1.5.820.122543.8.8.8"
    },
    {
      "name": "RSA",
      "type": "framework",
      "bom-ref": "i-made-this-up"
    },
    {
      "name": "AES-128-GCM",
      "type": "cryptographic-asset",
      "bom-ref": "crypto/algorithm/aes-128-gcm@2.16.840.1.101.3.4.1.6",
      "cryptoProperties": {
        "assetType": "algorithm",
        "algorithmProperties": {
          "parameterSetIdentifier": "128",
          "primitive": "ae",
          "mode": "gcm",
          "executionEnvironment": "software-plain-ram",
          "implementationPlatform": "x86_64",
          "cryptoFunctions": [ "keygen", "encrypt", "decrypt" ],
          "classicalSecurityLevel": 128,
          "nistQuantumSecurityLevel": 1
        },
        "oid": "2.16.840.1.101.3.4.1.6"
      }
    }
  ]
}`
	var bom cdx.BOM
	decoder := cdx.NewBOMDecoder(strings.NewReader(jsonBom), cdx.BOMFileFormatJSON)
	err := decoder.Decode(&bom)
	require.NoError(t, err)

	bomStats := service.BOMStats(context.Background(), &bom)

	require.Equal(t, 3, bomStats.Stats.CryptoAssets.Total)
	require.Equal(t, 2, bomStats.Stats.CryptoAssets.Algo.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Cert.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Protocol.Total)
	require.Equal(t, 1, bomStats.Stats.CryptoAssets.Related.Total)

}

func TestStatsComponentsNil(t *testing.T) {
	jsonBom := `{
  "bomFormat": "CycloneDX",
  "specVersion": "1.6",
  "serialNumber": "urn:uuid:e8c355aa-2142-4084-a8c7-6d42c8610ba2",
  "version": 1,
  "metadata": {
    "timestamp": "2024-01-09T12:00:00Z",
    "component": {
      "type": "application",
      "name": "my application",
      "version": "1.0"
    }
  }
}`
	var bom cdx.BOM
	decoder := cdx.NewBOMDecoder(strings.NewReader(jsonBom), cdx.BOMFileFormatJSON)
	err := decoder.Decode(&bom)
	require.NoError(t, err)

	bomStats := service.BOMStats(context.Background(), &bom)

	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Algo.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Cert.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Protocol.Total)
	require.Equal(t, 0, bomStats.Stats.CryptoAssets.Related.Total)

}
