package config

// Opinionated defaults for PandaBot
const (
	DefaultPort            = 31337
	CureThresholdPercent   = 70
	CuragaThresholdCount   = 3
	NaRemovalEnabled       = true
	DisableCuresDefault    = false
	IsPowerlevelingDefault = false

	HealthThresholdCritical = 25
	HealthThresholdLow      = 75
)

// Config holds the opinionated settings
type Config struct {
	Port             int
	CureThreshold    int
	CuragaThreshold  int
	NaRemovalEnabled bool
	DisableCures     bool
	IsPowerleveling  bool
	HealthThresholds struct {
		Critical int
		Low      int
	}
}

// Get returns the opinionated configuration
func Get() *Config {
	return &Config{
		Port:             DefaultPort,
		CureThreshold:    CureThresholdPercent,
		CuragaThreshold:  CuragaThresholdCount,
		NaRemovalEnabled: NaRemovalEnabled,
		DisableCures:     DisableCuresDefault,
		IsPowerleveling:  IsPowerlevelingDefault,
		HealthThresholds: struct {
			Critical int
			Low      int
		}{
			Critical: HealthThresholdCritical,
			Low:      HealthThresholdLow,
		},
	}
}
