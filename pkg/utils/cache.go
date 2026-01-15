// Package utils 缓存工具
package utils

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// Cache 全局缓存实例
var Cache *cache.Cache

func init() {
	// 默认过期时间 5 分钟，清理间隔 10 分钟
	Cache = cache.New(5*time.Minute, 10*time.Minute)
}

// CacheGet 获取缓存
func CacheGet(key string) (interface{}, bool) {
	return Cache.Get(key)
}

// CacheSet 设置缓存
func CacheSet(key string, value interface{}, duration time.Duration) {
	Cache.Set(key, value, duration)
}

// CacheDelete 删除缓存
func CacheDelete(key string) {
	Cache.Delete(key)
}

// CacheFlush 清空缓存
func CacheFlush() {
	Cache.Flush()
}

// CacheGetOrSet 获取或设置缓存
func CacheGetOrSet(key string, duration time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	if val, found := Cache.Get(key); found {
		return val, nil
	}

	val, err := fn()
	if err != nil {
		return nil, err
	}

	Cache.Set(key, val, duration)
	return val, nil
}
