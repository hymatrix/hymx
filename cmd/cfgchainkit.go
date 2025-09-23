package main

import (
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/spf13/viper"
)

func LoadChainkitConfig() chainkitSchema.Config {
	return chainkitSchema.Config{
		RedisUrl: viper.GetString("chainkit.redisURL"),
		Keyfile:  viper.GetString("chainkit.keyfilePath"),
		OptType:  viper.GetString("chainkit.optType"),
	}
}
