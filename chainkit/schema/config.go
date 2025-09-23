package schema

type Config struct {
	RedisUrl string `json:"redisUrl" yaml:"redisUrl"`
	Keyfile  string `json:"keyfile" yaml:"keyfile"`
	OptType  string `json:"optType" yaml:"optType"`
}
