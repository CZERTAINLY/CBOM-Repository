package service

import (
	"context"
	"log/slog"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type BomStats struct {
	Stats CryptoStats `json:"crypto-stats"`
}

type CryptoStats struct {
	CryptoAssets CryptoAssetStats `json:"crypto-assets"`
}

type CryptoAssetStats struct {
	Total    int        `json:"total"`
	Algo     TotalStats `json:"algorithms"`
	Cert     TotalStats `json:"certificates"`
	Protocol TotalStats `json:"protocols"`
	Related  TotalStats `json:"related-crypto-materials"`
}

type TotalStats struct {
	Total int `json:"total"`
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
			continue
		}
		bomStats.Stats.CryptoAssets.Total += 1

		switch component.CryptoProperties.AssetType {
		case cdx.CryptoAssetTypeAlgorithm:
			bomStats.Stats.CryptoAssets.Algo.Total += 1

		case cdx.CryptoAssetTypeCertificate:
			bomStats.Stats.CryptoAssets.Cert.Total += 1

		case cdx.CryptoAssetTypeProtocol:
			bomStats.Stats.CryptoAssets.Protocol.Total += 1

		case cdx.CryptoAssetTypeRelatedCryptoMaterial:
			bomStats.Stats.CryptoAssets.Related.Total += 1
		}
	}
	return bomStats
}
