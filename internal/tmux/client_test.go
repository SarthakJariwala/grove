package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestListSessionsParsesOutput(t *testing.T) {
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		return helperCommand(t, "session_ok")
	})
	defer restore()

	client := &Client{}
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	first := sessions[0]
	if first.Name != "api/one" || first.Windows != 3 || !first.Attached {
		t.Fatalf("first session parsed incorrectly: %#v", first)
	}
	if !first.HasAlerts || !first.AlertsBell || !first.AlertsActivity || first.AlertsSilence {
		t.Fatalf("first alert flags parsed incorrectly: %#v", first)
	}
	if first.LastActivity != 1710000000 {
		t.Fatalf("first.LastActivity = %d, want %d", first.LastActivity, int64(1710000000))
	}
}

func TestListSessionsNoServerRunningReturnsEmpty(t *testing.T) {
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		return helperCommand(t, "session_no_server")
	})
	defer restore()

	client := &Client{}
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(sessions) = %d, want 0", len(sessions))
	}
}

func TestListPanesParsesOutput(t *testing.T) {
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		return helperCommand(t, "panes_ok")
	})
	defer restore()

	client := &Client{}
	panes, err := client.ListPanes()
	if err != nil {
		t.Fatalf("ListPanes() error = %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("len(panes) = %d, want 2", len(panes))
	}

	p := panes[0]
	if p.SessionName != "api/one" || p.WindowIndex != 0 || p.Command != "go" {
		t.Fatalf("pane parsed incorrectly: %#v", p)
	}
	if !p.PaneActive || !p.WindowActive || !p.ActivityFlag || !p.BellFlag || p.SilenceFlag {
		t.Fatalf("pane flags parsed incorrectly: %#v", p)
	}
}

func TestListPanesNoServerRunningReturnsEmpty(t *testing.T) {
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		return helperCommand(t, "panes_no_server")
	})
	defer restore()

	client := &Client{}
	panes, err := client.ListPanes()
	if err != nil {
		t.Fatalf("ListPanes() error = %v", err)
	}
	if len(panes) != 0 {
		t.Fatalf("len(panes) = %d, want 0", len(panes))
	}
}

func TestMutatingCommandsIncludeTmuxOutputOnError(t *testing.T) {
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		return helperCommand(t, "mutate_error")
	})
	defer restore()

	client := &Client{}
	err := client.NewSession("api/one", "/tmp")
	if err == nil {
		t.Fatalf("NewSession() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error %q does not include tmux output", err.Error())
	}
}

func TestNewSessionWithCommandIncludesStartupCommand(t *testing.T) {
	var gotArgs []string
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		gotArgs = append([]string(nil), args...)
		return helperCommand(t, "mutate_ok")
	})
	defer restore()

	client := &Client{}
	if err := client.NewSessionWithCommand("api/cmd-start", "/tmp/api", "make start"); err != nil {
		t.Fatalf("NewSessionWithCommand() error = %v", err)
	}

	want := []string{"new-session", "-d", "-s", "api/cmd-start", "-c", "/tmp/api", "make start"}
	if got := fmt.Sprint(gotArgs); got != fmt.Sprint(want) {
		t.Fatalf("tmux args = %v, want %v", gotArgs, want)
	}
}

func TestActivePaneStates(t *testing.T) {
	t.Parallel()

	panes := []PaneInfo{
		{SessionName: "api/one", ActivityFlag: true},
		{SessionName: "api/one", WindowActive: true, PaneActive: true, Command: "go", PaneTitle: "* Claude", CurrentPath: "/tmp/api", BellFlag: true},
		{SessionName: "web/two", WindowActive: false, PaneActive: false, SilenceFlag: true},
	}

	states := ActivePaneStates(panes)
	if len(states) != 1 {
		t.Fatalf("len(states) = %d, want 1", len(states))
	}

	st := states["api/one"]
	if st.Command != "go" || st.PaneTitle != "Claude" || st.CurrentPath != "/tmp/api" {
		t.Fatalf("active pane metadata incorrect: %#v", st)
	}
	if !st.BellFlag || !st.ActivityFlag {
		t.Fatalf("active pane flags incorrect: %#v", st)
	}
}

func TestSessionWindowIndexes(t *testing.T) {
	t.Parallel()

	panes := []PaneInfo{
		{SessionName: "api/one", WindowIndex: 2},
		{SessionName: "api/one", WindowIndex: 0},
		{SessionName: "api/one", WindowIndex: 2},
		{SessionName: "api/one", WindowIndex: 5},
		{SessionName: "web/two", WindowIndex: 1},
	}

	indexes := SessionWindowIndexes(panes)
	if got := fmt.Sprint(indexes["api/one"]); got != "[0 2 5]" {
		t.Fatalf("api/one windows = %s, want [0 2 5]", got)
	}
	if got := fmt.Sprint(indexes["web/two"]); got != "[1]" {
		t.Fatalf("web/two windows = %s, want [1]", got)
	}
}

func TestActiveWindowIndexes(t *testing.T) {
	t.Parallel()

	panes := []PaneInfo{
		{SessionName: "api/one", WindowIndex: 2, WindowActive: true},
		{SessionName: "api/one", WindowIndex: 1, WindowActive: false},
		{SessionName: "web/two", WindowIndex: 3, WindowActive: false},
	}

	active := ActiveWindowIndexes(panes)
	if got, ok := active["api/one"]; !ok || got != 2 {
		t.Fatalf("api/one active window = %d, ok=%v, want 2,true", got, ok)
	}
	if _, ok := active["web/two"]; ok {
		t.Fatalf("web/two should not have an active window entry")
	}
}

func stubExecCommand(t *testing.T, fn func(name string, args ...string) *exec.Cmd) func() {
	t.Helper()
	old := execCommand
	execCommand = fn
	return func() { execCommand = old }
}

func helperCommand(t *testing.T, scenario string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", scenario)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	i := 0
	for i < len(args) && args[i] != "--" {
		i++
	}
	if i+1 >= len(args) {
		fmt.Fprintln(os.Stderr, "missing helper scenario")
		os.Exit(2)
	}

	switch args[i+1] {
	case "session_ok":
		fmt.Fprint(os.Stdout, "api/one:3:attached:!#:1710000000\nweb/two:1:detached::1700000000\n")
		os.Exit(0)
	case "session_no_server":
		fmt.Fprint(os.Stderr, "no server running on /tmp/tmux.sock\n")
		os.Exit(1)
	case "panes_ok":
		fmt.Fprint(os.Stdout, "api/one\t0\tgo\t1\t1\t1\t1\t0\t* Claude\t/tmp/api\nweb/two\t1\tzsh\t0\t0\t0\t0\t1\tmy-host\t/tmp/web\n")
		os.Exit(0)
	case "panes_no_server":
		fmt.Fprint(os.Stderr, "no current client\n")
		os.Exit(1)
	case "mutate_error":
		fmt.Fprint(os.Stderr, "permission denied\n")
		os.Exit(1)
	case "mutate_ok":
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper scenario: %s\n", args[i+1])
		os.Exit(2)
	}
}
