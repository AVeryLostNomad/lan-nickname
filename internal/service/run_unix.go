//go:build !windows

package service

import "fmt"

func Run(_ string) error {
	return fmt.Errorf("the internal service runner is only used by Windows Service Manager")
}
