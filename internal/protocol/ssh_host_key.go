package protocol

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const maxSSHHostKeyLength = 1024

var supportedSSHHostKeyTypes = map[string]struct{}{
	"ecdsa-sha2-nistp256": {},
	"ssh-ed25519":         {},
	"ssh-rsa":             {},
}

func ParseSSHHostKey(contents []byte) (string, error) {
	fields := strings.Fields(string(contents))
	if len(fields) < 2 {
		return "", errors.New("SSH host public key must contain a key type and key")
	}
	key := fields[0] + " " + fields[1]
	if err := ValidateSSHHostKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func ValidateSSHHostKey(key string) error {
	if key == "" {
		return nil
	}
	if len(key) > maxSSHHostKeyLength {
		return fmt.Errorf("SSH host public key exceeds %d bytes", maxSSHHostKeyLength)
	}
	fields := strings.Fields(key)
	if len(fields) != 2 || key != fields[0]+" "+fields[1] {
		return errors.New("SSH host public key is not normalized")
	}
	if _, supported := supportedSSHHostKeyTypes[fields[0]]; !supported {
		return fmt.Errorf("unsupported SSH host key type %q", fields[0])
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return errors.New("SSH host public key is not valid base64")
	}
	if len(blob) < 4 {
		return errors.New("SSH host public key blob is truncated")
	}
	nameLength := int(binary.BigEndian.Uint32(blob[:4]))
	if nameLength != len(fields[0]) || 4+nameLength > len(blob) || string(blob[4:4+nameLength]) != fields[0] {
		return errors.New("SSH host public key type does not match its key blob")
	}
	return nil
}
