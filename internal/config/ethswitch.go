package config

type EthSwitch struct {
	BaseURL           string
	CertPath          string
	KeyPath           string
	CAPath            string
	EthSwitchSFTPPath string `conf:"default:/data/clearing"`
}
