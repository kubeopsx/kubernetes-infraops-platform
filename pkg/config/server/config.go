package server

type Config struct {
	RPCAddress           string                      `yaml:"RPCAddress"`
	HTTPAddress          string                      `yaml:"HTTPAddress"`
	Mysql                []*MysqlConfig              `yaml:"mysql"`
	PublicCloudAssetSync *PublicCloudAssetSyncConfig `yaml:"publicCloudAssetSync"`
	InvertedIndex        []*InvertedIndexConfig      `yaml:"invertedIndex"`
	Probe                []*ProbeConfig              `yaml:"probe"`
}

type MysqlConfig struct {
	Name  string `yaml:"name"`
	Addr  string `yaml:"addr"`
	Max   int    `yaml:"max"`
	Idle  int    `yaml:"idle"`
	Debug bool   `yaml:"debug"`
}

type PublicCloudAssetSyncConfig struct {
	Enabled bool `yaml:"enabled"`
}

type InvertedIndexConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ResourceName string `yaml:"resource_name"`
	Modulus      int    `yaml:"modulus"`
	Num          int    `yaml:"num"`
}

type ProbeConfig struct {
	Type            string            `yaml:"type"`
	Region          string            `yaml:"region"`
	Target          []string          `yaml:"target"`
	IntervalSeconds int               `yaml:"interval_seconds"`
	TimeoutSeconds  int               `yaml:"timeout_seconds"`
	Options         map[string]string `yaml:"options"`
}
