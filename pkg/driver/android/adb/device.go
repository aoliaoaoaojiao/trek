package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Device struct {
	client Client
	serial string
	attrs  map[string]string
}

func (d Device) Product() string {
	return d.attrs["product"]
}

func (d Device) Model() string {
	return d.attrs["model"]
}

func (d Device) Usb() string {
	return d.attrs["usb"]
}

func (d Device) DeviceInfo() map[string]string {
	return d.attrs
}

func (d Device) Serial() string {
	return d.serial
}

func (d Device) IsUsb() bool {
	return d.Usb() != ""
}

func (d Device) State(ctx context.Context) (DeviceState, error) {
	resp, err := d.client.run(ctx, d.serial, "get-state")
	return deviceStateConv(resp), err
}

func (d Device) DevicePath(ctx context.Context) (string, error) {
	resp, err := d.client.run(ctx, d.serial, "get-devpath")
	return resp, err
}

func (d Device) ForwardLocalAbstract(ctx context.Context, localPort int, remotePort string, noRebind ...bool) (err error) {
	local := fmt.Sprintf("tcp:%d", localPort)
	remote := fmt.Sprintf("localabstract:%s", remotePort)
	return d.forward(ctx, local, remote, noRebind...)
}

func (d Device) ForwardTcp(ctx context.Context, localPort int, remotePort int, noRebind ...bool) (err error) {
	local := fmt.Sprintf("tcp:%d", localPort)
	remote := fmt.Sprintf("tcp:%d", remotePort)
	return d.forward(ctx, local, remote, noRebind...)
}

func (d Device) forward(ctx context.Context, local, remote string, noRebind ...bool) (err error) {
	args := []string{"forward"}
	if len(noRebind) != 0 && noRebind[0] {
		args = append(args, "--no-rebind")
	}
	args = append(args, local, remote)
	_, err = d.client.run(ctx, d.serial, args...)
	return
}

func (d Device) ForwardList() (deviceForwardList []DeviceForward, err error) {
	var forwardList []DeviceForward
	if forwardList, err = d.client.ForwardList(); err != nil {
		return nil, err
	}

	deviceForwardList = make([]DeviceForward, 0, len(deviceForwardList))
	for i := range forwardList {
		if forwardList[i].Serial == d.serial {
			deviceForwardList = append(deviceForwardList, forwardList[i])
		}
	}
	return
}

func (d Device) ForwardKill(ctx context.Context, localPort int) (err error) {
	local := fmt.Sprintf("tcp:%d", localPort)
	_, err = d.client.run(ctx, d.serial, "forward", "--remove", local)
	return
}

func (d Device) ReverseLocalAbstract(ctx context.Context, remotePort string, localPort int, noRebind ...bool) (err error) {
	local := fmt.Sprintf("tcp:%d", localPort)
	remote := fmt.Sprintf("localabstract:%s", remotePort)
	return d.reverse(ctx, remote, local, noRebind...)
}

func (d Device) ReverseTcp(ctx context.Context, remotePort, localPort int, noRebind ...bool) (err error) {
	local := fmt.Sprintf("tcp:%d", localPort)
	remote := fmt.Sprintf("tcp:%d", remotePort)
	return d.reverse(ctx, remote, local, noRebind...)
}

func (d Device) reverse(ctx context.Context, remote, local string, noRebind ...bool) (err error) {
	args := []string{"reverse"}
	if len(noRebind) != 0 && noRebind[0] {
		args = append(args, "--no-rebind")
	}
	args = append(args, remote, local)
	_, err = d.client.run(ctx, d.serial, args...)
	return
}

func (d Device) ReverseList(ctx context.Context) (deviceForward []DeviceForward, err error) {
	var resp string
	if resp, err = d.client.run(ctx, d.serial, "reverse", "--list"); err != nil {
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
			deviceForward = append(deviceForward, DeviceForward{Serial: d.serial, Remote: fields[0], Local: fields[1]})
		}
	}
	return
}

func (d Device) ReverseKillLocalAbstract(ctx context.Context, remotePort string) (err error) {
	remote := fmt.Sprintf("localabstract:%s", remotePort)
	return d.reverseKill(ctx, remote)
}

func (d Device) ReverseKillTcp(ctx context.Context, localPort int) (err error) {
	remote := fmt.Sprintf("tcp:%d", localPort)
	return d.reverseKill(ctx, remote)
}

func (d Device) reverseKill(ctx context.Context, remote string) (err error) {
	_, err = d.client.run(ctx, d.serial, "reverse", "--remove", remote)
	return
}

func (d Device) ReverseKillAll(ctx context.Context) (err error) {
	_, err = d.client.run(ctx, d.serial, "reverse", "--remove-all")
	return
}

func (d Device) RunShellCommand(ctx context.Context, cmd string, args ...string) (string, error) {
	raw, err := d.RunShellCommandWithBytes(ctx, cmd, args...)
	return string(raw), err
}

func (d Device) RunShellCommandWithBytes(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}
	if strings.TrimSpace(cmd) == "" {
		return nil, errors.New("adb shell: command cannot be empty")
	}

	executable, err := d.client.resolveExecutable()
	if err != nil {
		return nil, err
	}

	shellArgs := []string{"shell", cmd}
	if d.serial != "" {
		shellArgs = append([]string{"-s", d.serial}, shellArgs...)
	}

	shellCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	command := exec.CommandContext(shellCtx, executable, shellArgs...)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("adb shell failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("adb shell failed: %w", err)
	}
	return output, nil
}

func (d Device) GetCurrentPackage(ctx context.Context) (string, error) {
	output, err := d.RunShellCommand(ctx, "dumpsys", "activity", "top")
	if err != nil {
		return "", err
	}
	token, err := parseTopActivityToken(output)
	if err != nil {
		return "", err
	}
	pkg, _, err := splitActivityToken(token)
	if err != nil {
		return "", err
	}
	return pkg, nil
}

func (d Device) GetCurrentActivity(ctx context.Context) (string, error) {
	output, err := d.RunShellCommand(ctx, "dumpsys", "activity", "top")
	if err != nil {
		return "", err
	}
	token, err := parseTopActivityToken(output)
	if err != nil {
		return "", err
	}
	_, activity, err := splitActivityToken(token)
	if err != nil {
		return "", err
	}
	return activity, nil
}

func parseTopActivityToken(output string) (string, error) {
	lines := strings.Split(output, "\n")
	var lastActivityLine string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "ACTIVITY") {
			lastActivityLine = line
		}
	}
	if lastActivityLine == "" {
		return "", errors.New("no current activity found")
	}
	parts := strings.Fields(lastActivityLine)
	for _, part := range parts {
		if strings.Contains(part, "/") {
			return part, nil
		}
	}
	return "", errors.New("no current activity found")
}

func splitActivityToken(token string) (pkg string, activity string, err error) {
	parts := strings.SplitN(token, "/", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid activity token")
	}
	pkg = strings.TrimSpace(parts[0])
	activity = strings.TrimSpace(parts[1])
	if pkg == "" || activity == "" {
		return "", "", errors.New("invalid activity token")
	}
	if strings.HasPrefix(activity, ".") {
		activity = pkg + activity
	}
	return pkg, activity, nil
}

func (d Device) RunShellLoopCommandSock(ctx context.Context, cmd string, args ...string) (net.Conn, error) {
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}
	if strings.TrimSpace(cmd) == "" {
		return nil, errors.New("adb shell: command cannot be empty")
	}

	executable, err := d.client.resolveExecutable()
	if err != nil {
		return nil, err
	}

	shellArgs := []string{"shell", cmd}
	if d.serial != "" {
		shellArgs = append([]string{"-s", d.serial}, shellArgs...)
	}

	cmdExec := exec.CommandContext(ctx, executable, shellArgs...)

	stdin, err := cmdExec.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe failed: %w", err)
	}

	stdout, err := cmdExec.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("create stdout pipe failed: %w", err)
	}

	stderr, err := cmdExec.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("create stderr pipe failed: %w", err)
	}

	if err := cmdExec.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("start shell process failed: %w", err)
	}

	conn := &shellConn{
		cmd:    cmdExec,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	return conn, nil
}

type shellConn struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.Reader
	stderr        io.Reader
	mu            sync.Mutex
	readDeadline  time.Time
	writeDeadline time.Time
}

func (c *shellConn) Read(b []byte) (n int, err error) {
	if d := c.getReadDeadline(); !d.IsZero() && time.Now().After(d) {
		return 0, &net.OpError{Op: "read", Net: "pipe", Err: os.ErrDeadlineExceeded}
	}
	return c.stdout.Read(b)
}

func (c *shellConn) Write(b []byte) (n int, err error) {
	if d := c.getWriteDeadline(); !d.IsZero() && time.Now().After(d) {
		return 0, &net.OpError{Op: "write", Net: "pipe", Err: os.ErrDeadlineExceeded}
	}
	return c.stdin.Write(b)
}

func (c *shellConn) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	return nil
}

func (c *shellConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "shell", Net: "pipe"}
}

func (c *shellConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "device", Net: "pipe"}
}

func (c *shellConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.mu.Unlock()
	c.scheduleKill(t)
	return nil
}

func (c *shellConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.mu.Unlock()
	c.scheduleKill(t)
	return nil
}

func (c *shellConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeDeadline = t
	c.mu.Unlock()
	c.scheduleKill(t)
	return nil
}

func (c *shellConn) getReadDeadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readDeadline
}

func (c *shellConn) getWriteDeadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeDeadline
}

// scheduleKill 在 deadline 到期时终止进程，使阻塞中的 Read/Write 返回错误。
func (c *shellConn) scheduleKill(t time.Time) {
	if t.IsZero() || c.cmd == nil || c.cmd.Process == nil {
		return
	}
	if d := time.Until(t); d > 0 {
		time.AfterFunc(d, func() {
			if c.cmd != nil && c.cmd.Process != nil {
				c.cmd.Process.Kill()
			}
		})
	}
}

func (d Device) EnableAdbOverTCP(ctx context.Context, port ...int) (err error) {
	if len(port) == 0 {
		port = []int{AdbDaemonPort}
	}

	_, err = d.client.run(ctx, d.serial, "tcpip", fmt.Sprintf("%d", port[0]))
	return
}

func (d Device) List(ctx context.Context, remotePath string) (devFileInfos []DeviceFileInfo, err error) {
	output, err := d.RunShellCommand(ctx, "ls", "-la", remotePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	devFileInfos = make([]DeviceFileInfo, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		info := DeviceFileInfo{}
		info.Name = strings.Join(fields[5:], " ")
		if _, err := fmt.Sscanf(fields[4], "%d", &info.Size); err == nil {
			devFileInfos = append(devFileInfos, info)
		}
	}

	return
}

func (d Device) PushFile(ctx context.Context, local *os.File, remotePath string, modification ...time.Time) (err error) {
	if len(modification) == 0 {
		var stat os.FileInfo
		if stat, err = local.Stat(); err != nil {
			return err
		}
		modification = []time.Time{stat.ModTime()}
	}

	return d.Push(ctx, local, remotePath, modification[0], DefaultFileMode)
}

func (d Device) Push(ctx context.Context, source io.Reader, remotePath string, _ time.Time, mode ...os.FileMode) (err error) {
	if len(mode) == 0 {
		mode = []os.FileMode{DefaultFileMode}
	}

	tmpFile, err := os.CreateTemp("", "adb-push-*")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, source); err != nil {
		return fmt.Errorf("write temp file failed: %w", err)
	}

	tmpFile.Close()

	executable, err := d.client.resolveExecutable()
	if err != nil {
		return err
	}

	args := []string{"push", tmpFile.Name(), normalizeRemotePath(remotePath)}
	if d.serial != "" {
		args = append([]string{"-s", d.serial}, args...)
	}

	pushCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(pushCtx, executable, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb push failed: %s: %w", string(output), err)
	}

	return nil
}

func (d Device) Pull(ctx context.Context, remotePath string, dest io.Writer) (err error) {
	executable, err := d.client.resolveExecutable()
	if err != nil {
		return err
	}

	args := []string{"exec-out", "cat", normalizeRemotePath(remotePath)}
	if d.serial != "" {
		args = append([]string{"-s", d.serial}, args...)
	}

	pullCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(pullCtx, executable, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adb pull failed: %s: %w", stderr.String(), err)
	}

	_, err = dest.Write(stdout.Bytes())
	return err
}

func normalizeRemotePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func (d Device) Install(ctx context.Context, apkPath string, replace bool) error {
	args := []string{"install"}
	if replace {
		args = append(args, "-r")
	}
	args = append(args, filepath.Clean(apkPath))

	output, err := d.client.run(ctx, d.serial, args...)
	if err != nil {
		return err
	}

	if !strings.Contains(strings.ToLower(output), "success") {
		return fmt.Errorf("adb install failed: %s", output)
	}

	return nil
}

func (d Device) Uninstall(ctx context.Context, packageName string, keepData bool) error {
	args := []string{"uninstall"}
	if keepData {
		args = append(args, "-k")
	}
	args = append(args, packageName)

	output, err := d.client.run(ctx, d.serial, args...)
	if err != nil {
		return err
	}

	if !strings.Contains(strings.ToLower(output), "success") {
		return fmt.Errorf("adb uninstall failed: %s", output)
	}

	return nil
}

func (d Device) Screenshot(ctx context.Context) ([]byte, error) {
	executable, err := d.client.resolveExecutable()
	if err != nil {
		return nil, err
	}

	args := []string{"exec-out", "screencap", "-p"}
	if d.serial != "" {
		args = append([]string{"-s", d.serial}, args...)
	}

	screenCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(screenCtx, executable, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("screenshot failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return output, nil
}
