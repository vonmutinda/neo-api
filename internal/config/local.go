package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/ardanlabs/conf/v3"
	"github.com/joho/godotenv"
)

func LoadConf[T *API](confToLoad T) (T, error) {
	prefix := EnvConfigPrefix
	if err := loadEnv(); err != nil {
		return nil, err
	}
	if _, parseErr := conf.Parse(prefix, confToLoad); parseErr != nil {
		if errors.Is(parseErr, conf.ErrHelpWanted) {
			usage, usageErr := conf.String(confToLoad)
			if usageErr != nil {
				return nil, fmt.Errorf("error getting usage: %w", usageErr)
			}
			fmt.Println(usage)
			return nil, nil
		}
		return nil, fmt.Errorf("error parsing config: %w", parseErr)
	}
	return confToLoad, nil
}

func loadEnv() error {
	filename := ".env.local"
	if _, err := os.Stat(filename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return godotenv.Load(filename)
}
