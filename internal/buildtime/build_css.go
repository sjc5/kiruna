package buildtime

import (
	"fmt"

	"github.com/sjc5/kiruna/internal/common"
)

func BuildCSS(config *common.Config) error {
	err := ProcessCSS(config, "critical")
	if err != nil {
		return fmt.Errorf("error processing critical CSS: %v", err)
	}
	err = ProcessCSS(config, "normal")
	if err != nil {
		return fmt.Errorf("error processing normal CSS: %v", err)
	}
	return nil
}
