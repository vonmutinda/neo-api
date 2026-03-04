package config

type Formance struct {
	URL           string `conf:"default:http://localhost:3068"`
	LedgerName    string `conf:"default:neobank"`
	AccountPrefix string `conf:"default:neo"`
	ClientID      string
	ClientSecret  string
}
