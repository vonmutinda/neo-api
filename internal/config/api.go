package config

import (
	"github.com/ardanlabs/conf/v3"
)

const EnvConfigPrefix = "NEO"

type API struct {
	conf.Version
	AppInfo           *AppInfo
	Database          *Database
	Log               *Log
	Formance          *Formance
	EthSwitch         *EthSwitch
	Fayda             *Fayda
	Web               *Web
	CorsSettings      *CorsSettings
	Telegram          *Telegram
	RateLimit         *RateLimit
	JWT               *JWT
	AdminJWT          *AdminJWT
	OpenExchangeRates *OpenExchangeRatesConfig
	Wise              *WiseConfig
	Redis             *RedisConfig
}

type AdminJWT struct {
	Secret   string
	Issuer   string `conf:"default:neobank-admin"`
	Audience string `conf:"default:neobank-admin-api"`
}

func (c *API) IsProduction() bool {
	return c.AppInfo.Environment == "production"
}

type AppInfo struct {
	AppVersion  conf.Version
	AppName     string `conf:"default:neobank-api"`
	Environment string `conf:"default:development"`
}

func LoadAPI() (*API, error) {
	apiConf := new(API)
	return LoadConf(apiConf)
}
