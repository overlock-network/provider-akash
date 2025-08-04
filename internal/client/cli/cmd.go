package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (c AkashCommand) AsCmd() (*exec.Cmd, error) {
	if len(c.Content) == 0 {
		return nil, errors.New("empty command")
	}

	path, err := exec.LookPath(c.Content[0])
	if err != nil {
		return nil, err
	}

	switch c.Content[0] {
	case "akash":
		// #nosec
		return exec.Command(path, c.Headless()...), nil
	case "provider-services":
		// #nosec
		return exec.Command(path, c.Headless()...), nil
	default:
		return nil, fmt.Errorf("invalid command: %s", c.Content[0])
	}
}

type AkashErrorResponse struct {
	RawLog string `json:"raw_log"`
}

func (c AkashCommand) Raw() ([]byte, error) {
	cmd, err := c.AsCmd()
	if err != nil {
		return nil, err
	}

	strings.Join(cmd.Args, " ")

	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Could not execute command: %s", err.Error())
		if strings.Contains(errb.String(), "error unmarshalling") {
			return c.Raw()
		}

		var akErr AkashErrorResponse
		err := json.Unmarshal(out, &akErr)
		if err != nil {
			fmt.Printf("Failure unmarshalling error: %s", err)
		}

		if strings.Contains(akErr.RawLog, "out of gas in location") {
			return nil, errors.New(akErr.RawLog)
		}

		return nil, errors.New(errb.String())
	}

	fmt.Printf("Output: %s", out)

	return out, nil
}

func (c AkashCommand) DecodeJson(v any) error {
	cmd, err := c.AsCmd()
	if err != nil {
		return err
	}

	strings.Join(cmd.Args, " ")

	var errb bytes.Buffer
	cmd.Stderr = &errb
	fmt.Println(cmd)
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		if strings.Contains(errb.String(), "error unmarshalling") {
			return c.DecodeJson(v)
		}

		return errors.New(errb.String())
	}

	fmt.Printf("Output: %s", out)

	err = json.NewDecoder(strings.NewReader(string(out))).Decode(v)
	if err != nil {
		fmt.Printf("Error while unmarshalling command output")
		return err
	}

	return nil
}
