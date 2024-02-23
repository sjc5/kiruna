package common

import "os"

type KirunaEnv string

var KirunaEnvDevelopment KirunaEnv = "development"

func SetKirunaEnvDev() {
	os.Setenv("KIRUNA_ENV", string(KirunaEnvDevelopment))
}

func GetIsKirunaEnvDev() bool {
	return os.Getenv("KIRUNA_ENV") == string(KirunaEnvDevelopment)
}
