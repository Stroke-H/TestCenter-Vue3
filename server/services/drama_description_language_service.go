package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"testcenter-server/dramalanguage"
)

var dramaDescriptionLanguageMu sync.Mutex

// Shared by interactive, scheduled and second-confirmation execution paths.
// The local cache stores predictions only, not descriptions or credentials.
func prepareDramaDescriptionLanguages(ctx context.Context, rootDir string, message func(string)) error {
	dramaDescriptionLanguageMu.Lock()
	defer dramaDescriptionLanguageMu.Unlock()
	started := time.Now()
	filename := filepath.Join(rootDir, "drama_info.json")
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}
	var ids []string
	var metadata map[string]map[string]json.RawMessage
	if err := json.Unmarshal(payload["dramaList"], &ids); err != nil {
		return err
	}
	if err := json.Unmarshal(payload["dramaMeta"], &metadata); err != nil {
		return err
	}
	cachePath := filepath.Join(rootDir, "server", "data", "drama_description_language_cache.json")
	var cache struct {
		Version string                             `json:"version"`
		Items   map[string]dramalanguage.Detection `json:"items"`
	}
	if data, readErr := os.ReadFile(cachePath); readErr == nil {
		if json.Unmarshal(data, &cache) != nil {
			cache.Version = ""
		}
	}
	if cache.Version != dramalanguage.CacheVersion || cache.Items == nil {
		cache.Version, cache.Items = dramalanguage.CacheVersion, map[string]dramalanguage.Detection{}
	}
	counts := map[string]int{}
	hits := 0
	nextCache := map[string]dramalanguage.Detection{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		meta := metadata[id]
		if meta == nil {
			return fmt.Errorf("剧集 %s 缺少元数据，无法完成简介语言检查", id)
		}
		var description, language string
		var descriptionAvailable, langAvailable bool
		if err := json.Unmarshal(meta["description_available"], &descriptionAvailable); err != nil || !descriptionAvailable {
			return fmt.Errorf("剧集 %s 的 description 未成功获取，不能当成空简介跳过", id)
		}
		if err := json.Unmarshal(meta["lang_available"], &langAvailable); err != nil || !langAvailable {
			return fmt.Errorf("剧集 %s 的 lang 未成功获取，无法完成简介语种检查", id)
		}
		if _, ok := meta["description"]; !ok {
			return fmt.Errorf("缺少 description 字段，请使用新版 prepare_data.js 重新准备数据")
		}
		if err := json.Unmarshal(meta["description"], &description); err != nil {
			return fmt.Errorf("剧集 %s description 字段类型异常", id)
		}
		if err := json.Unmarshal(meta["lang"], &language); err != nil {
			return fmt.Errorf("剧集 %s lang 字段类型异常", id)
		}
		key := dramalanguage.DescriptionHash(description)
		detection, found := cache.Items[key]
		if found {
			hits++
		} else {
			detection = dramalanguage.Detect(description)
			cache.Items[key] = detection
		}
		nextCache[key] = detection
		result := dramalanguage.Compare(description, language, detection)
		if detection.Method == "arabic_transliteration_rules" {
			counts["transliterated"]++
		}
		if detection.Method == "arabic_transliteration_candidate" {
			counts["transliteration_uncertain"]++
		}
		counts[result.Status]++
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		meta["description_language_check"] = encoded
	}
	encodedMeta, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	payload["dramaMeta"] = encodedMeta
	validation, _ := json.Marshal(map[string]interface{}{"ready": true, "version": dramalanguage.CacheVersion,
		"counts": counts, "total": len(ids), "cache_hits": hits, "duration_ms": time.Since(started).Milliseconds()})
	payload["descriptionLanguageValidation"] = validation
	updated, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := writeDramaLanguageJSON(filename, updated); err != nil {
		return err
	}
	cache.Items = nextCache
	cacheContent, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if err := writeDramaLanguageJSON(cachePath, cacheContent); err != nil {
		log.Printf("[DramaDescriptionLanguage] cache save failed: %v", err)
	}
	summary := fmt.Sprintf("[简介语种] 检查完成：%d 部，缓存命中 %d，通过 %d，空简介跳过 %d，无法确定/待复核 %d，疑似混合 %d，疑似范围外 %d，转写候选 %d，疑似不一致 %d，语言字段异常 %d，耗时 %s",
		len(ids), hits, counts["passed"], counts["skipped"], counts["uncertain"], counts["mixed"], counts["unsupported"], counts["transliterated"], counts["mismatch"], counts["invalid_lang"], time.Since(started).Round(time.Millisecond))
	if message != nil {
		message(summary)
	} else {
		log.Print(summary)
	}
	return nil
}

func writeDramaLanguageJSON(path string, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".drama-language-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
