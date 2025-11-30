package service

import (
	"context"
	"log/slog"
	"slices"
	"strconv"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type BomStats struct {
	Stats CryptoStats `json:"cryptoStats"`
}

type CryptoStats struct {
	Asset AssetStats `json:"assets"`
	Algo  AlgoStats  `json:"algorithms"`
}

type AssetStats struct {
	Total         int               `json:"total"`
	Certs         CertStats         `json:"certificates"`
	Keys          KeyStats          `json:"keys"`
	HashFns       HashFnStats       `json:"hashFunctions"`
	SignSchemes   SignSchemeStats   `json:"signatureSchemes"`
	KeyAgreements KeyAgreementStats `json:"keyAgreement"`
}

type CertStats struct {
	Total int `json:"total"`
}

type KeyStats struct {
	Total int `json:"total"`
	Sym   int `json:"symmetric"`
	Asym  int `json:"asymmetric"`
}

type HashFnStats struct {
	Total int `json:"total"`
}

type SignSchemeStats struct {
	Total int `json:"signatureSchemes"`
}

type KeyAgreementStats struct {
	Total int `json:"keyAgreements"`
}

type AlgoStats struct {
	Unique   []string `json:"unique"`
	KeySizes []int    `json:"keySizes"`
	Count    int      `json:"count"`
}

func BOMStats(ctx context.Context, bom *cdx.BOM) BomStats {
	var bomStats BomStats
	if bom.Components == nil {
		slog.WarnContext(ctx, "BOM has nil root level 'Components' field.", slog.String("serialNumber", bom.SerialNumber))
		return bomStats
	}

	for _, component := range *bom.Components {
		if component.Type != cdx.ComponentTypeCryptographicAsset {
			continue
		}
		if component.CryptoProperties == nil {
			slog.WarnContext(ctx, "Component is a crypto asset but has a nil CryptoProperties field. Skipping.", slog.String("bom-ref", component.BOMRef))
			bomStats.Stats.Asset.Total += 1
			continue
		}
		switch component.CryptoProperties.AssetType {
		case cdx.CryptoAssetTypeAlgorithm:
			statsAlgo(ctx, &bomStats.Stats.Algo, component)

		case cdx.CryptoAssetTypeCertificate:
			statsCert(ctx, &bomStats.Stats.Asset, component)

		case cdx.CryptoAssetTypeProtocol:
			statsProtocol(ctx, &bomStats.Stats.Asset, component)

		case cdx.CryptoAssetTypeRelatedCryptoMaterial:
			statsRelated(ctx, &bomStats.Stats.Asset, component)

		}
	}
	return bomStats
}

func statsRelated(ctx context.Context, as *AssetStats, c cdx.Component) {
	as.Total += 1
	switch c.CryptoProperties.RelatedCryptoMaterialProperties.Type {
	case cdx.RelatedCryptoMaterialTypePrivateKey:
		as.Keys.Total += 1
		as.Keys.Asym += 1

	case cdx.RelatedCryptoMaterialTypePublicKey:
		as.Keys.Total += 1
		as.Keys.Asym += 1

	case cdx.RelatedCryptoMaterialTypePassword:
		fallthrough
	case cdx.RelatedCryptoMaterialTypeCredential:
		fallthrough
	case cdx.RelatedCryptoMaterialTypeToken:
		fallthrough
	case cdx.RelatedCryptoMaterialTypeSharedSecret:
		fallthrough
	case cdx.RelatedCryptoMaterialTypeSecretKey:
		fallthrough
	case cdx.RelatedCryptoMaterialTypeKey:
		as.Keys.Total += 1
		as.Keys.Sym += 1

	case cdx.RelatedCryptoMaterialTypeSignature:
		as.SignSchemes.Total += 1

	// EDU: don't know where to put the following values
	case cdx.RelatedCryptoMaterialTypeDigest:
	case cdx.RelatedCryptoMaterialTypeInitializationVector:
	case cdx.RelatedCryptoMaterialTypeNonce:
	case cdx.RelatedCryptoMaterialTypeSeed:
	case cdx.RelatedCryptoMaterialTypeSalt:
	case cdx.RelatedCryptoMaterialTypeTag:
	case cdx.RelatedCryptoMaterialTypeAdditionalData:
	case cdx.RelatedCryptoMaterialTypeCiphertext:
	}
}

func statsProtocol(_ context.Context, as *AssetStats, c cdx.Component) {
	as.Total += 1
	switch c.CryptoProperties.ProtocolProperties.Type {
	case cdx.CryptoProtocolTypeTLS:
		fallthrough
	case cdx.CryptoProtocolTypeSSH:
		fallthrough
	case cdx.CryptoProtocolTypeIPSec:
		fallthrough
	case cdx.CryptoProtocolTypeIKE:
		fallthrough
	case cdx.CryptoProtocolTypeSSTP:
		fallthrough
	case cdx.CryptoProtocolTypeWPA:
		as.KeyAgreements.Total += 1
	}
}

func statsCert(_ context.Context, as *AssetStats, _ cdx.Component) {
	as.Total += 1
	as.Certs.Total += 1
}

func statsAlgo(ctx context.Context, as *AlgoStats, c cdx.Component) {
	as.Count += 1
	if !slices.Contains(as.Unique, c.Name) {
		as.Unique = append(as.Unique, c.Name)
	}
	i, err := strconv.ParseInt(c.CryptoProperties.AlgorithmProperties.ParameterSetIdentifier, 10, 64)
	if err != nil || i < 0 {
		slog.WarnContext(ctx, "Field ParameterSetIdentifier should be a number.",
			slog.String("bom-ref", c.BOMRef),
			slog.String("parameterSetIdentifier", c.CryptoProperties.AlgorithmProperties.ParameterSetIdentifier))
	} else if !slices.Contains(as.KeySizes, int(i)) {
		as.KeySizes = append(as.KeySizes, int(i))
	}

}
