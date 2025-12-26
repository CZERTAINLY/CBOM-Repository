package service

import (
	"context"
	"log/slog"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type CryptoStats struct {
	CryptoAsset CryptoAssetStats `json:"cryptoAssets"`
}

type CryptoAssetStats struct {
	Total    int        `json:"total"`
	Algo     TotalStats `json:"algorithms"`
	Cert     TotalStats `json:"certificates"`
	Protocol TotalStats `json:"protocols"`
	Related  TotalStats `json:"relatedCryptoMaterials"`
}

type TotalStats struct {
	Total int `json:"total"`
}

func CalculateCryptoStats(ctx context.Context, bom *cdx.BOM) CryptoStats {
	var cryptoStats CryptoStats
	if bom.Components == nil {
		slog.WarnContext(ctx, "BOM has nil root level 'Components' field.", slog.String("serialNumber", bom.SerialNumber))
		return cryptoStats
	}

	for _, component := range *bom.Components {
		if component.Type != cdx.ComponentTypeCryptographicAsset {
			continue
		}

		if component.CryptoProperties == nil {
			slog.WarnContext(ctx, "Component is a crypto asset but has a nil CryptoProperties field. Skipping.", slog.String("bom-ref", component.BOMRef))
			continue
		}
		cryptoStats.CryptoAsset.Total += 1

		switch component.CryptoProperties.AssetType {
		case cdx.CryptoAssetTypeAlgorithm:
			cryptoStats.CryptoAsset.Algo.Total += 1

		case cdx.CryptoAssetTypeCertificate:
			cryptoStats.CryptoAsset.Cert.Total += 1

		case cdx.CryptoAssetTypeProtocol:
			cryptoStats.CryptoAsset.Protocol.Total += 1

		case cdx.CryptoAssetTypeRelatedCryptoMaterial:
			cryptoStats.CryptoAsset.Related.Total += 1
		}
	}
	return cryptoStats
}
