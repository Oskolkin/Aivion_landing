package landscape

func deriveCategoryBlocks(feed *CoreFeed, profile ExecutableProfileV3) {
	if feed == nil {
		return
	}
	for i := range feed.Categories {
		c := &feed.Categories[i]
		catID := c.Identity.CategoryID
		if c.Identity.Unmapped {
			catID = UnmappedCategoryID
		}
		ensureCategoryDerivedBlocks(c, catID, feed.SKUs, profile, feed.GlobalCapabilities)
	}
}

func ensureCategoryDerivedBlocks(cl *CategoryLandscape, catID int64, skus []SKULandscape, profile ExecutableProfileV3, caps []CapabilityFlag) {
	wantTraffic := false
	if spec, ok := profile.Block("category.advertising.traffic"); ok && capabilityReadyV3(caps, spec) {
		wantTraffic = true
	}
	wantAttr := false
	if spec, ok := profile.Block("category.advertising.attribution"); ok && capabilityReadyV3(caps, spec) {
		wantAttr = true
	}
	legacyAds := profile.HasBlock("category.advertising")
	if cl.Advertising == nil && (wantTraffic || wantAttr || legacyAds) {
		cl.Advertising = rollupCategoryAdsV4(catID, skus)
		if cl.Advertising == nil {
			cl.Advertising = &CategoryAdsBlock{State: BlockReadyEmpty}
		}
		if wantTraffic && cl.Advertising.Traffic == nil {
			cl.Advertising.Traffic = &CategoryAdsTrafficBlock{State: BlockReadyEmpty}
		}
		if wantAttr && cl.Advertising.Attribution == nil {
			cl.Advertising.Attribution = &CategoryAdsAttributionBlock{State: BlockReadyEmpty}
		}
	}
	if profile.HasBlock("category.stock") && cl.Stock == nil {
		cl.Stock = rollupCategoryStock(catID, skus, nil, nil)
	}
	if profile.HasBlock("category.supply") && cl.Supply == nil {
		cl.Supply = rollupCategorySupply(catID, skus)
	}
	if profile.HasBlock("category.price") && cl.Price == nil {
		cl.Price = rollupCategoryPrice(catID, skus)
	}
}
