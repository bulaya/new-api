package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureDefaultModelMetadataCreatesSeeds(t *testing.T) {
	db := setupDefaultModelMetaTestDB(t)

	require.NoError(t, EnsureDefaultModelMetadata())

	var gpt54Mini Model
	require.NoError(t, db.Where("model_name = ?", "gpt-5.4-mini").First(&gpt54Mini).Error)
	require.Equal(t, "响应快、成本低，适合高频轻量任务", gpt54Mini.Description)
	require.Equal(t, "经济", gpt54Mini.Tags)
	require.Equal(t, NameRuleExact, gpt54Mini.NameRule)
	require.Equal(t, 0, gpt54Mini.SyncOfficial)

	var gpt54 Model
	require.NoError(t, db.Where("model_name = ?", "gpt-5.4").First(&gpt54).Error)
	require.Equal(t, "综合能力强，适合复杂推理、代码与高质量内容生成", gpt54.Description)
	require.Equal(t, "全能", gpt54.Tags)

	var openAI Vendor
	require.NoError(t, db.Where("name = ?", "OpenAI").First(&openAI).Error)
	require.Equal(t, openAI.Id, gpt54Mini.VendorID)
	require.Equal(t, openAI.Id, gpt54.VendorID)
}

func TestEnsureDefaultModelMetadataDoesNotOverrideExistingModel(t *testing.T) {
	db := setupDefaultModelMetaTestDB(t)

	custom := &Model{
		ModelName:    "gpt-5.4",
		Description:  "自定义说明",
		Tags:         "自定义",
		Status:       1,
		SyncOfficial: 0,
		NameRule:     NameRuleExact,
	}
	require.NoError(t, custom.Insert())

	require.NoError(t, EnsureDefaultModelMetadata())

	var reloaded Model
	require.NoError(t, db.Where("model_name = ?", "gpt-5.4").First(&reloaded).Error)
	require.Equal(t, "自定义说明", reloaded.Description)
	require.Equal(t, "自定义", reloaded.Tags)

	var count int64
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "gpt-5.4").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestEnsureDefaultModelMetadataFillsMissingFields(t *testing.T) {
	db := setupDefaultModelMetaTestDB(t)

	incomplete := &Model{
		ModelName:    "gpt-5.4-mini",
		Description:  "",
		Tags:         "",
		Status:       1,
		SyncOfficial: 0,
	}
	require.NoError(t, incomplete.Insert())

	require.NoError(t, EnsureDefaultModelMetadata())

	var reloaded Model
	require.NoError(t, db.Where("model_name = ?", "gpt-5.4-mini").First(&reloaded).Error)
	require.Equal(t, "响应快、成本低，适合高频轻量任务", reloaded.Description)
	require.Equal(t, "经济", reloaded.Tags)

	var openAI Vendor
	require.NoError(t, db.Where("name = ?", "OpenAI").First(&openAI).Error)
	require.Equal(t, openAI.Id, reloaded.VendorID)
}

func TestSortRuleModelsBySpecificity(t *testing.T) {
	models := []*Model{
		{Id: 3, ModelName: "gpt-5.4"},
		{Id: 2, ModelName: "gpt-5.4-mini"},
		{Id: 1, ModelName: "gpt"},
	}

	sortRuleModelsBySpecificity(models)

	require.Equal(t, "gpt-5.4-mini", models[0].ModelName)
	require.Equal(t, "gpt-5.4", models[1].ModelName)
	require.Equal(t, "gpt", models[2].ModelName)
}

func setupDefaultModelMetaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db

	require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Ability{}, &Channel{}))

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}
