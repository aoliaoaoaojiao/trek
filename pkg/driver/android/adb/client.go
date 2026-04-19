package adb

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Client struct {
	executablePath string
	platform       string
}

func NewClient() (Client, error) {
	return NewClientWith("")
}

func NewClientWith(executablePath string, _ ...int) (Client, error) {
	client := Client{
		executablePath: executablePath,
		platform:       runtime.GOOS,
	}

	path, err := client.resolveExecutable()
	if err != nil {
		return Client{}, err
	}

	return Client{
		executablePath: path,
		platform:       runtime.GOOS,
	}, nil
}

func (c Client) resolveExecutable() (string, error) {
	if strings.TrimSpace(c.executablePath) != "" {
		return c.executablePath, nil
	}
	name := "adb"
	if c.platform == "windows" {
		name = "adb.exe"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("adb executable not found: %w", err)
	}
	return path, nil
}

func (c Client) run(ctx context.Context, serial string, args ...string) (string, error) {
	executable, err := c.resolveExecutable()
	if err != nil {
		return "", err
	}

	commandArgs := make([]string, 0, len(args)+2)
	if strings.TrimSpace(serial) != "" {
		commandArgs = append(commandArgs, "-s", serial)
	}
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, executable, commandArgs...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))

	if err != nil {
		return "", fmt.Errorf("adb command failed: %s: %w", result, err)
	}

	return result, nil
}

func (c Client) ServerVersion() (version int, err error) {
	var resp string
	if resp, err = c.run(context.Background(), "", "version"); err != nil {
		return 0, err
	}

	lines := strings.Split(resp, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Android Debug Bridge version") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				var v int64
				if v, err = strconv.ParseInt(fields[4], 10, 64); err != nil {
					return 0, fmt.Errorf("parse version failed: %w", err)
				}
				version = int(v)
				return
			}
		}
	}
	return 0, fmt.Errorf("version not found in output: %s", resp)
}

func (c Client) DeviceSerialList() (serials []string, err error) {
	var resp string
	if resp, err = c.run(context.Background(), "", "devices"); err != nil {
		return
	}

	lines := strings.Split(resp, "\n")
	serials = make([]string, 0, len(lines))

	for i := range lines {
		fields := strings.Fields(lines[i])
		if len(fields) >= 2 {
			serials = append(serials, fields[0])
		}
	}

	return
}

func (c Client) DeviceList() (devices []Device, err error) {
	var resp string
	if resp, err = c.run(context.Background(), "", "devices", "-l"); err != nil {
		return
	}

	lines := strings.Split(resp, "\n")
	devices = make([]Device, 0, len(lines))

	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) == 0 {
			continue
		}

		sliceAttrs := fields[2:]
		mapAttrs := map[string]string{}
		for _, field := range sliceAttrs {
			split := strings.Split(field, ":")
			if len(split) >= 2 {
				key, val := split[0], split[1]
				mapAttrs[key] = val
			}
		}
		devices = append(devices, Device{client: c, serial: fields[0], attrs: mapAttrs})
	}

	return
}

func (c Client) ForwardList() (deviceForward []DeviceForward, err error) {
	var resp string
	if resp, err = c.run(context.Background(), "", "forward", "--list"); err != nil {
		return nil, err
	}

	lines := strings.Split(resp, "\n")
	deviceForward = make([]DeviceForward, 0, len(lines))

	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			deviceForward = append(deviceForward, DeviceForward{Serial: fields[0], Local: fields[1], Remote: fields[2]})
		}
	}

	return
}

func (c Client) ForwardKillAll() (err error) {
	_, err = c.run(context.Background(), "", "forward", "--remove-all")
	return
}

func (c Client) Connect(ip string, port ...int) (err error) {
	if len(port) == 0 {
		port = []int{AdbDaemonPort}
	}

	var resp string
	address := fmt.Sprintf("%s:%d", ip, port[0])
	if resp, err = c.run(context.Background(), "", "connect", address); err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "connected to") && !strings.HasPrefix(resp, "already connected to") {
		return fmt.Errorf("adb connect: %s", resp)
	}
	return
}

func (c Client) Disconnect(ip string, port ...int) (err error) {
	var address string
	if len(port) != 0 {
		address = fmt.Sprintf("%s:%d", ip, port[0])
	} else {
		address = ip
	}

	var resp string
	if resp, err = c.run(context.Background(), "", "disconnect", address); err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "disconnected") {
		return fmt.Errorf("adb disconnect: %s", resp)
	}
	return
}

func (c Client) DisconnectAll() (err error) {
	var resp string
	if resp, err = c.run(context.Background(), "", "disconnect"); err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "disconnected everything") && !strings.Contains(resp, "disconnected") {
		return fmt.Errorf("adb disconnect all: %s", resp)
	}
	return
}

func (c Client) KillServer() (err error) {
	_, err = c.run(context.Background(), "", "kill-server")
	return
}

func (c Client) StartServer() (err error) {
	_, err = c.run(context.Background(), "", "start-server")
	return
}
