package schema

import goarSchema "github.com/permadao/goar/schema"

type Config struct {
	RedisUrl     string `json:"redisUrl" yaml:"redisUrl"`
	NodeRedisUrl string `json:"nodeRedisUrl" yaml:"nodeRedisUrl"`
	Keyfile      string `json:"keyfile" yaml:"keyfile"`
	OptType      string `json:"optType" yaml:"optType"`
}

type DownloadResult struct {
	Nonce      int64
	Assignment *goarSchema.BundleItem
	Message    *goarSchema.BundleItem
}
