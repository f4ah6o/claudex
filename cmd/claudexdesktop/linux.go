//go:build linux

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

const (
	linuxDesktopCommandEnv     = "CLAUDEX_DESKTOP_COMMAND"
	linuxDesktopProcessEnv     = "CLAUDEX_DESKTOP_PROCESS_NAME"
	linuxDesktopModeEnv        = "CLAUDEX_DESKTOP_MODE"
	linuxGatewayBaseURLEnv     = "CLAUDEX_GATEWAY_BASE_URL"
	linuxGatewayAPIKeyEnv      = "CLAUDEX_GATEWAY_API_KEY"
	linuxGatewayAuthSchemeEnv  = "CLAUDEX_GATEWAY_AUTH_SCHEME"
	linuxInferenceModelsEnv    = "CLAUDEX_INFERENCE_MODELS"
	linuxDesktopCommandDefault = "claude-desktop"
	linuxDesktopProcessDefault = "claude-desktop"
)

func runLinux() error {
	configPath := resolveConfigPath()
	if absolutePath, errAbs := filepath.Abs(configPath); errAbs == nil {
		configPath = absolutePath
	}
	templatePath := resolveResourcePath(templateFileName)
	created, err := ensureConfig(configPath, templatePath)
	if err != nil {
		return err
	}
	if created {
		_ = showMessage("ClaudexDesktop setup", fmt.Sprintf("Created %s.\n\nRun Codex login once, then open ClaudexDesktop again:\n\n%s login --config %s", configPath, resolveResourcePath("claudex-server"), configPath), true)
		return nil
	}

	cfg, _, err := claudex.LoadServeConfig(configPath)
	if err != nil {
		return err
	}
	localKey, err := claudex.LocalAPIKey(cfg)
	if err != nil {
		return err
	}
	if !hasAuthMaterial(cfg.AuthDir) {
		return fmt.Errorf("Codex authentication is not configured; run %s login --config %s first", resolveResourcePath("claudex-server"), configPath)
	}

	if isLinuxDesktopRunning() {
		confirmed, errConfirm := confirmLinuxRestart()
		if errConfirm != nil {
			return errConfirm
		}
		if !confirmed {
			return nil
		}
		if err = quitLinuxDesktop(); err != nil {
			return err
		}
	}

	if _, err = startServer(cfg, configPath); err != nil {
		return err
	}
	if err = openLinuxDesktop(cfg, localKey); err != nil {
		return err
	}
	if !waitForLinuxDesktop(true, 120) {
		return fmt.Errorf("Claude Desktop did not start; check the Linux Desktop launcher and ~/.claudex/desktop.log")
	}
	waitForLinuxDesktopExit()
	return nil
}

func openLinuxDesktop(cfg *config.Config, localKey string) error {
	command := linuxDesktopCommand()
	cmd := exec.Command(command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = linuxDesktopEnvironment(os.Environ(), cfg, localKey)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Claude Desktop with %s: %w", command, err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func linuxDesktopCommand() string {
	if command := strings.TrimSpace(os.Getenv(linuxDesktopCommandEnv)); command != "" {
		return command
	}
	return linuxDesktopCommandDefault
}

func linuxDesktopProcessName() string {
	if name := strings.TrimSpace(os.Getenv(linuxDesktopProcessEnv)); name != "" {
		return name
	}
	return linuxDesktopProcessDefault
}

func linuxDesktopEnvironment(base []string, cfg *config.Config, localKey string) []string {
	environment := append([]string(nil), base...)
	environment = setEnvironmentValue(environment, linuxDesktopModeEnv, "1")
	environment = setEnvironmentValue(environment, linuxGatewayBaseURLEnv, claudex.ServerURL(cfg))
	environment = setEnvironmentValue(environment, linuxGatewayAPIKeyEnv, localKey)
	environment = setEnvironmentValue(environment, linuxGatewayAuthSchemeEnv, "bearer")
	environment = setEnvironmentValue(environment, linuxInferenceModelsEnv, claudex.InferenceModelsValue())
	return environment
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, prefix+value)
}

func linuxDesktopPIDs() ([]int, error) {
	pgrep, err := exec.LookPath("pgrep")
	if err != nil {
		return nil, fmt.Errorf("find pgrep: %w", err)
	}
	output, err := exec.Command(pgrep, "-x", "--", linuxDesktopProcessName()).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("find Claude Desktop process: %w", err)
	}

	fields := strings.Fields(string(output))
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, errParse := strconv.Atoi(field)
		if errParse != nil || pid <= 0 {
			return nil, fmt.Errorf("parse Claude Desktop process ID %q", field)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func isLinuxDesktopRunning() bool {
	pids, err := linuxDesktopPIDs()
	return err == nil && len(pids) > 0
}

func confirmLinuxRestart() (bool, error) {
	message := "Claude Desktop is running. Restart it in Claudex mode?"
	if dialog, err := exec.LookPath("kdialog"); err == nil {
		if confirmed, dialogErr := runLinuxConfirmationDialog(dialog, "--title", "ClaudexDesktop", "--yesno", message); dialogErr == nil {
			return confirmed, nil
		}
	}
	if dialog, err := exec.LookPath("zenity"); err == nil {
		if confirmed, dialogErr := runLinuxConfirmationDialog(dialog, "--question", "--title=ClaudexDesktop", "--text="+message); dialogErr == nil {
			return confirmed, nil
		}
	}

	stdinInfo, errIn := os.Stdin.Stat()
	stdoutInfo, errOut := os.Stdout.Stat()
	interactive := errIn == nil && errOut == nil && stdinInfo.Mode()&os.ModeCharDevice != 0 && stdoutInfo.Mode()&os.ModeCharDevice != 0
	if !interactive {
		return false, errors.New("Claude Desktop is running; install zenity or kdialog, or close Claude Desktop before retrying")
	}

	fmt.Fprint(os.Stderr, message+" [y/N] ")
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read restart confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func runLinuxConfirmationDialog(command string, args ...string) (bool, error) {
	err := exec.Command(command, args...).Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func quitLinuxDesktop() error {
	pids, err := linuxDesktopPIDs()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		process, errFind := os.FindProcess(pid)
		if errFind != nil {
			return fmt.Errorf("find Claude Desktop process %d: %w", pid, errFind)
		}
		if errSignal := process.Signal(syscall.SIGTERM); errSignal != nil {
			return fmt.Errorf("ask Claude Desktop process %d to quit: %w", pid, errSignal)
		}
	}
	if !waitForLinuxDesktop(false, 120) {
		return errors.New("Claude Desktop did not exit after the restart request")
	}
	return nil
}

func waitForLinuxDesktop(running bool, attempts int) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		if isLinuxDesktopRunning() == running {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func waitForLinuxDesktopExit() {
	for isLinuxDesktopRunning() {
		time.Sleep(time.Second)
	}
}

func showMessageLinux(title, message string, informational bool) error {
	if dialog, err := exec.LookPath("kdialog"); err == nil {
		args := []string{"--title", title}
		if informational {
			args = append(args, "--msgbox", message)
		} else {
			args = append(args, "--error", message)
		}
		if err = exec.Command(dialog, args...).Run(); err == nil {
			return nil
		}
	}
	if dialog, err := exec.LookPath("zenity"); err == nil {
		kind := "--error"
		if informational {
			kind = "--info"
		}
		if err = exec.Command(dialog, kind, "--title="+title, "--text="+message).Run(); err == nil {
			return nil
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
	return nil
}
