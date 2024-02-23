package buildtime

import "github.com/sjc5/kiruna/internal/common"

func BuildCSS(config *common.Config) error {
	err := ProcessCSS(config, "critical")
	if err != nil {
		return err
	}
	err = ProcessCSS(config, "normal")
	if err != nil {
		return err
	}
	return nil
}
