// Package launchctl contains some experimental code for working with the
// macOS launchctl system. Use at your own risk.
package launchctl

import (
	"strconv"
	"time"
)

// Config is a representation of the launchctl config for an agent.
//
// See https://www.manpagez.com/man/5/launchd.plist
type Config struct {
	// The unique identifier for the job (required).
	Label string `json:"label"`

	// Indicates that the job should not be loaded.
	Disabled bool `json:"disabled,omitempty"`

	// The user to run the job as. This is only applicable when launchd is
	// running as root.
	UserName string `json:"userName,omitempty"`

	// The group to run the job as. This is only applicable when launchd is
	// running as root.
	GroupName string `json:"groupName,omitempty"`

	// This configuration only applies to sessions of this type.
	// Types are: "Aqua" (default), "Background", and "LoginWindow".
	SessionType string `json:"sessionType,omitempty"`

	// The name of the program to execute. If omitted, the first element of
	// ProgramArguments will be used instead.
	Program string `json:"program,omitempty"`

	// The complete argument list of the program to execute. This is required if
	// Program is not set. The first element of this slice, if any, becomes
	// argv[0] in the executed process.
	ProgramArguments []string `json:"programArguments,omitempty"`

	// If true, update the program arguments by glob expansion before executing.
	EnableGlobbing bool `json:"enableGlobbing,omitempty"`

	// Whether the job should be kept continuously running or triggered on
	// demand. The default is false (on-demand).
	KeepAlive *KeepAliveConfig `json:"keepAlive,omitempty"`

	// Whether the job is launched once when it is first loaded.
	RunAtLoad bool `json:"runAtLoad,omitempty"`

	// An optional directory to chroot to before running the job.
	RootDirectory string `json:"rootDirectory,omitempty"`

	// An optional directory to chdir to before running the job.
	WorkingDirectory string `json:"workingDirectory,omitempty"`

	// A map of environment variables to set on the running process.
	Environment map[string]string `json:"environment,omitempty"`

	// A set of file paths which, if any is modified, the job is started.
	WatchPaths []string `json:"watchPaths,omitempty"`

	// A set of directory paths which, if any is non-empty, the job is started.
	QueuePaths []string `json:"queuePaths,omitempty"`

	// If true, start the job whenever a filesystem is mounted.
	StartOnMount bool `json:"startOnMount,omitempty"`

	// If positive, start the job at this interval.
	StartInterval DurationSec `json:"startInterval,omitempty"`

	// If set, plumb this file as stdin to the job.
	StandardInPath string `json:"stdinPath,omitempty"`

	// If set, plumb stdout from the job to this file.
	StandardOutPath string `json:"stdoutPath,omitempty"`

	// If set, plumb stderr from the job to this file.
	StandardErrorPath string `json:"stderrPath,omitempty"`

	// A process type hint to the system.  Options are "Background", "Standard",
	// "Adaptive", and "Interactive". The default is "Standard".
	ProcessType string `json:"processType,omitempty"`

	// Omitted:
	//  - inetdCompatibility (dict)
	//  - LimitLoadToHosts (array of string)
	//  - LimitLoadFromHosts (array of strings)
	//  - EnableTransactions (bool)
	//  - OnDemand (bool; deprecated)
	//  - Umask (int)
	//  - TimeOut (int; seconds)
	//  - ExitTimeOut (int; seconds)
	//  - ThrottleInterval (int; seconds)
	//  - InitGroups (bool)
	//  - StartCalendarInterval (dict)
	//  - Debug (bool)
	//  - WaitForDebugger (bool)
	//  - SoftResourceLimits (dict)
	//  - HardResourceLimits (dict)
	//  - Nice (int)
	//  - AbandonProcessGroup (bool)
	//  - LowPriorityIO (bool)
	//  - LaunchOnlyOnce (bool)
	//  - MachServices (dict)
	//  - Sockets (dict-of-dict)
}

type DurationSec time.Duration

func (d DurationSec) MarshalJSON() ([]byte, error) {
	sec := time.Duration(d) / time.Second
	return strconv.AppendInt(nil, int64(sec), 10), nil
}

func (d *DurationSec) UnmarshalJSON(data []byte) error {
	v, err := strconv.Atoi(string(data))
	if err != nil {
		return err
	}
	*d = DurationSec(time.Duration(v) * time.Second)
	return nil
}

type KeepAliveConfig struct {
	// If true, the job will be kept running unconditionally always, and the
	// remaining settings are ignored.
	Always bool `json:"always,omitempty"`

	// If non-nil, restart the job when the program exits.
	// If true, restart when the program exits successfully.
	// Otherwise restart when the program exits unsuccessfully.
	SuccessfulExit *bool `json:"successfulExit,omitempty"`

	// If non-nil, run the job according to the current network state.
	// If true, run when the network is up (at least one non-loopback).
	// Otherwise, run when the network is down.
	NetworkState *bool `json:"networkUp,omitempty"`

	// A collection of file paths. If the value for a path is true, the path
	// must exist for the job to be kept alive; if the value is false, the path
	// must not exist for the job to be kept alive.
	PathState map[string]bool `json:"pathState,omitempty"`

	// Omitted:
	//   - OtherJobEnabled
}
