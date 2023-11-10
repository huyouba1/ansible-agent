package conf

import "github.com/huyouba1/ansible-agent/ansible"

type Config struct {
	SSL      SSLSection          `toml:"ssl"`
	Ldap     ansible.LdapOptions `toml:"ldap"`
	HttpAuth HttpAuth            `toml:"http"`
}

type SSLSection struct {
	Enabled     bool `toml:"enabled"`
	Certificate string
	PrivateKey  string `toml:"private_key"`
	ClientCA    string `toml:"client_ca"`
}

type HttpAuth struct {
	Enabled bool   `toml:"enabled"`
	Token   string `toml:"token"`
}

func DefaultConfig() *Config {
	return &Config{}
}
