package config

import "flag"

type Config struct {
	InsecureListenAddress string
	Upstream              string
	TlsSkipVerify         bool
	BearerFile            string
	ForceGet              bool
	RestrictFile          string
}

func (cfg *Config) Load() {
	flag.StringVar(&cfg.InsecureListenAddress, "insecure-listen-address", "127.0.0.1:9099", "The address which proxy listens on")
	flag.StringVar(&cfg.Upstream, "upstream", "http://127.0.0.1:9090", "The upstream Thanos URL")
	flag.BoolVar(&cfg.TlsSkipVerify, "tlsSkipVerify", false, "Skip TLS Verification")
	flag.StringVar(&cfg.BearerFile, "bearer-file", "", "File containing bearer token for API requests")
	flag.BoolVar(&cfg.ForceGet, "force-get", false, "Force api.Client to use GET by rejecting POST requests")
	flag.StringVar(&cfg.RestrictFile, "restrict-file", "", "File containing restricted metrics definitions")
	flag.Parse()
}
