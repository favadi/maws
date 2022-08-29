package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

const (
	awsCliCmd            = "aws"
	applicationName      = "maws"
	sessionTokenFileName = "session-token.json"
	awsTimeLayout        = "2006-01-02T15:04:05-07:00"
)

type awsTime time.Time

func (t *awsTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(*t).Format(awsTimeLayout) + `"`), nil
}

func (t *awsTime) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	parsed, err := time.Parse(awsTimeLayout, str)
	if err != nil {
		return err
	}
	*t = awsTime(parsed)
	return nil
}

func awsProfile() string {
	profile := os.Getenv("MAWS_PROFILE")
	if profile == "" {
		profile = "default"
	}
	return profile
}

type SessionToken struct {
	Credentials struct {
		AccessKeyId     string  `json:"AccessKeyId"`
		SecretAccessKey string  `json:"SecretAccessKey"`
		SessionToken    string  `json:"SessionToken"`
		Expiration      awsTime `json:"Expiration"`
	} `json:"Credentials"`
}

func (t *SessionToken) IsExpired() bool {
	return time.Time(t.Credentials.Expiration).Before(time.Now())
}

func (t *SessionToken) Env() []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=%s", "AWS_ACCESS_KEY_ID", t.Credentials.AccessKeyId))
	env = append(env, fmt.Sprintf("%s=%s", "AWS_SECRET_ACCESS_KEY", t.Credentials.SecretAccessKey))
	env = append(env, fmt.Sprintf("%s=%s", "AWS_SESSION_TOKEN", t.Credentials.SessionToken))
	return env
}

func (t *SessionToken) ExportEnvs() string {
	var sessionEnvs []string
	sessionEnvs = append(sessionEnvs, fmt.Sprintf("export AWS_ACCESS_KEY_ID='%s'", t.Credentials.AccessKeyId))
	sessionEnvs = append(sessionEnvs, fmt.Sprintf("export AWS_SECRET_ACCESS_KEY='%s'", t.Credentials.SecretAccessKey))
	sessionEnvs = append(sessionEnvs, fmt.Sprintf("export AWS_SESSION_TOKEN='%s'", t.Credentials.SessionToken))
	return strings.Join(sessionEnvs, "\n")
}

func sessionTokenFile() string {
	return filepath.Join(xdg.DataHome, applicationName, sessionTokenFileName)
}

func deleteSessionToken() error {
	return os.RemoveAll(sessionTokenFile())
}

func persistSessionToken(st SessionToken) error {
	dataDir := filepath.Join(xdg.DataHome, applicationName)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	f, err := os.Create(sessionTokenFile())
	if err != nil {
		return fmt.Errorf("create session token file: %w", err)
	}
	if err = json.NewEncoder(f).Encode(&st); err != nil {
		return fmt.Errorf("write session token: %w", err)
	}

	return nil
}

func loadSessionToken() (SessionToken, error) {
	f, err := os.Open(sessionTokenFile())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return promptMFA()
		}

		return SessionToken{}, fmt.Errorf("open session token file: %w", err)
	}
	defer f.Close()

	var st SessionToken
	if err = json.NewDecoder(f).Decode(&st); err != nil {
		return SessionToken{}, fmt.Errorf("decode session token file: %w", err)
	}

	if st.IsExpired() {
		return promptMFA()
	}

	return st, nil
}

func getMFASerialNumber() (string, error) {
	var out = bytes.NewBuffer(nil)
	cmd := exec.Command(awsCliCmd, "--profile", awsProfile(), "iam", "list-mfa-devices", "--output", "json")
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execute get-caller-identity command: %w", err)
	}

	var mfaDevices = struct {
		MFADevices []struct {
			SerialNumber string `json:"SerialNumber"`
		} `json:"MFADevices"`
	}{}

	if err := json.NewDecoder(out).Decode(&mfaDevices); err != nil {
		return "", fmt.Errorf("decode caller identity: %w", err)
	}

	if len(mfaDevices.MFADevices) == 0 {
		return "", fmt.Errorf("no MFA devices configured")
	}
	return mfaDevices.MFADevices[0].SerialNumber, nil
}

func promptMFA() (SessionToken, error) {
	fmt.Printf("OTP: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return SessionToken{}, fmt.Errorf("read OTP from stint: %w", err)
	}
	tokenCode := scanner.Text()

	serialNumber, err := getMFASerialNumber()
	if err != nil {
		return SessionToken{}, fmt.Errorf("get MFA serial number: %w", err)
	}

	var out = bytes.NewBuffer(nil)
	cmd := exec.Command(awsCliCmd, "--profile", awsProfile(), "sts", "get-session-token", "--serial-number", serialNumber, "--token-code", tokenCode)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return SessionToken{}, fmt.Errorf("get-session-token")
	}

	var st SessionToken
	if err = json.Unmarshal(out.Bytes(), &st); err != nil {
		return SessionToken{}, fmt.Errorf("decode session token: %w", err)
	}
	if err = persistSessionToken(st); err != nil {
		return SessionToken{}, fmt.Errorf("persist session token: %w", err)
	}

	return st, nil
}

func runAWSCli(st SessionToken) error {
	cmd := exec.Command(awsCliCmd, os.Args[1:]...)
	cmd.Env = st.Env()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run AWS CLI: %w", err)
	}
	return nil
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "delete-session-token" {
		if err := deleteSessionToken(); err != nil {
			log.Print(err.Error())
			os.Exit(1)
		}
		return
	}

	st, err := loadSessionToken()
	if err != nil {
		log.Print(err.Error())
		os.Exit(1)
	}

	if len(os.Args) == 2 && os.Args[1] == "export-envs" {
		fmt.Println(st.ExportEnvs())
		return
	}

	if err := runAWSCli(st); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		log.Print(err.Error())
		os.Exit(1)
	}
}
