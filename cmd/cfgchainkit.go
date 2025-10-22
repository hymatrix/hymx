package main

import (
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/spf13/viper"
)

func LoadChainkitConfig() (chainkitSchema.Config, bool) {
	if !viper.GetBool("enableChainkit") {
		return chainkitSchema.Config{}, false
	}
	return chainkitSchema.Config{
		RedisUrl: viper.GetString("chainkit.redisURL"),
		Keyfile:  viper.GetString("chainkit.keyfilePath"),
		OptType:  viper.GetString("chainkit.optType"),
	}, true
}
