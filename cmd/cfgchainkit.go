package main

import (
	"fmt"
	"strings"

	"github.com/hymatrix/hymx/chainkit"
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/spf13/viper"
)

func LoadChainkitConfig() (*chainkit.Chainkit, error) {
	cfg := chainkitSchema.Config{
		RedisUrl:     viper.GetString("chainkit.redisURL"),
		NodeRedisUrl: viper.GetString("chainkit.nodeRedisURL"),
		Keyfile:      viper.GetString("chainkit.keyfilePath"),
		OptType:      viper.GetString("chainkit.optType"),
	}

	var missing []string
	if cfg.RedisUrl == "" {
		missing = append(missing, "chainkit.redisURL")
	}
	if cfg.NodeRedisUrl == "" {
		missing = append(missing, "chainkit.nodeRedisURL")
	}
	if cfg.Keyfile == "" {
		missing = append(missing, "chainkit.keyfilePath")
	}
	if cfg.OptType == "" {
		missing = append(missing, "chainkit.optType")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required chainkit config: %s", strings.Join(missing, ", "))
	}

	return chainkit.New(cfg), nil
}
