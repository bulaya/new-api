package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type defaultModelMetaSeed struct {
	ModelName   string
	Description string
	Tags        string
	VendorName  string
	NameRule    int
}

var defaultModelMetaSeeds = []defaultModelMetaSeed{
	{
		ModelName:   "gpt-5.4-mini",
		Description: "响应快、成本低，适合高频轻量任务",
		Tags:        "经济",
		VendorName:  "OpenAI",
		NameRule:    NameRuleExact,
	},
	{
		ModelName:   "gpt-5.4",
		Description: "综合能力强，适合复杂推理、代码与高质量内容生成",
		Tags:        "全能",
		VendorName:  "OpenAI",
		NameRule:    NameRuleExact,
	},
}

func EnsureDefaultModelMetadata() error {
	if DB == nil {
		return nil
	}

	var vendors []Vendor
	if err := DB.Find(&vendors).Error; err != nil {
		return err
	}

	vendorMap := make(map[int]*Vendor, len(vendors))
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	created := false
	updated := false
	for _, seed := range defaultModelMetaSeeds {
		var existing Model
		lookupErr := DB.Where("model_name = ?", seed.ModelName).First(&existing).Error
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		vendorID, err := ensureDefaultModelVendor(seed.VendorName, vendorMap)
		if err != nil {
			return err
		}

		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			meta := &Model{
				ModelName:    seed.ModelName,
				Description:  seed.Description,
				Tags:         seed.Tags,
				VendorID:     vendorID,
				Status:       1,
				SyncOfficial: 0,
				NameRule:     seed.NameRule,
			}
			if err := meta.Insert(); err != nil {
				return err
			}
			created = true
			continue
		}

		updates := map[string]interface{}{}
		if strings.TrimSpace(existing.Description) == "" && seed.Description != "" {
			updates["description"] = seed.Description
		}
		if strings.TrimSpace(existing.Tags) == "" && seed.Tags != "" {
			updates["tags"] = seed.Tags
		}
		if existing.VendorID == 0 && vendorID != 0 {
			updates["vendor_id"] = vendorID
		}
		if len(updates) == 0 {
			continue
		}

		updates["updated_time"] = common.GetTimestamp()
		if err := DB.Model(&Model{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return err
		}
		updated = true
	}

	if created || updated {
		RefreshPricing()
	}
	return nil
}

func ensureDefaultModelVendor(vendorName string, vendorMap map[int]*Vendor) (int, error) {
	if vendorName == "" {
		return 0, nil
	}

	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id, nil
		}
	}

	now := common.GetTimestamp()
	vendor := &Vendor{
		Name:        vendorName,
		Icon:        getDefaultVendorIcon(vendorName),
		Status:      1,
		CreatedTime: now,
		UpdatedTime: now,
	}
	if err := DB.Create(vendor).Error; err != nil {
		return 0, err
	}
	vendorMap[vendor.Id] = vendor
	return vendor.Id, nil
}
