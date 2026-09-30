package filelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPSLstart_RunsPSInUTCAndTheCLocale pins the environment every
// `ps -o lstart=` exec runs in (SI-300 as amended, FL-R1): TZ=UTC0 and
// LC_ALL=C win over whatever locale this process inherits, and the output
// is read as UTC. The seam's command stands in for ps and prints a fixed
// lstart line only when it sees exactly that environment; the rows fix
// both of lstartLayouts' day forms.
func TestPSLstart_RunsPSInUTCAndTheCLocale(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("LANG", "de_DE.UTF-8")
	orig := lstartCommand
	t.Cleanup(func() { lstartCommand = orig })
	for _, tc := range []struct {
		line string
		want time.Time
	}{
		{"Tue Sep  1 21:58:50 2026", time.Date(2026, time.September, 1, 21, 58, 50, 0, time.UTC)},
		{"Tue Sep 1 21:58:50 2026", time.Date(2026, time.September, 1, 21, 58, 50, 0, time.UTC)},
		{"Wed Sep 30 01:58:50 2026", time.Date(2026, time.September, 30, 1, 58, 50, 0, time.UTC)},
	} {
		t.Run(tc.line, func(t *testing.T) {
			lstartCommand = func(ctx context.Context, _ int) *exec.Cmd {
				return exec.CommandContext(ctx, "sh", "-c", `[ "$TZ" = UTC0 ] && [ "$LC_ALL" = C ] && printf '%s\n' "$1"`, "sh", tc.line)
			}
			got, err := psLstart(os.Getpid())
			if err != nil {
				t.Fatalf("psLstart = %v, want the stand-in's line read (it answers only under TZ=UTC0 and LC_ALL=C)", err)
			}
			if got.Unix() != tc.want.Unix() {
				t.Fatalf("psLstart = %s (unix %d), want %s (unix %d): the line must be read as UTC", got, got.Unix(), tc.want, tc.want.Unix())
			}
		})
	}
	t.Run("a ps that fails is an error, never a guessed start", func(t *testing.T) {
		lstartCommand = func(ctx context.Context, _ int) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", "exit 1")
		}
		if got, err := psLstart(os.Getpid()); err == nil {
			t.Fatalf("psLstart = %s, nil, want an error", got)
		}
	})
	t.Run("localized output is an error, never a guessed start", func(t *testing.T) {
		lstartCommand = func(ctx context.Context, _ int) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", `printf '%s\n' 'Di. 29 Sep. 22:02:18 2026'`)
		}
		if got, err := psLstart(os.Getpid()); err == nil {
			t.Fatalf("psLstart = %s, nil, want an unparseable-output error", got)
		}
	})
}

// proberHelperEnv names the lock path a re-executed test binary probes when
// it runs as TestHelperLockProber's child process.
const proberHelperEnv = "VERDI_FILELOCK_TEST_PROBER_LOCK"

// TestHelperLockProber is not a test on its own: it is the prober child
// process runProberChild re-executes this test binary as, so the prober
// runs in a time zone and locale of its own. In that child it reads the
// lock named by proberHelperEnv and prints one line per verdict — its own
// `ps -o lstart=` reading of the recorded pid, Peek's, Inspect's, and what
// its own Acquire got (releasing the lock again if it took it over) — then
// exits.
func TestHelperLockProber(t *testing.T) {
	path := os.Getenv(proberHelperEnv)
	if path == "" {
		t.Skip("helper child process only; driven by runProberChild")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("error: reading the lock: %v\n", err)
		os.Exit(3)
	}
	var info Info
	if err := strictUnmarshal(data, &info); err != nil {
		fmt.Printf("error: decoding the lock %q: %v\n", string(data), err)
		os.Exit(3)
	}
	if st, err := psLstart(info.PID); err != nil {
		fmt.Printf("reading error: %v\n", err)
	} else {
		fmt.Printf("reading %d\n", st.Unix())
	}
	if _, held, err := Peek(path); err != nil {
		fmt.Printf("peek error: %v\n", err)
	} else {
		fmt.Printf("peek %t\n", held)
	}
	if insp, err := Inspect(path); err != nil {
		fmt.Printf("inspect error: %v\n", err)
	} else {
		fmt.Printf("inspect %s\n", insp.Status)
		fmt.Printf("inspect-reason %s\n", insp.Reason)
	}
	f, err := Acquire(path)
	var held *ErrHeld
	switch {
	case err == nil:
		fmt.Println("acquire took-over")
		_ = Release(f, path)
	case errors.As(err, &held):
		fmt.Println("acquire held")
	default:
		fmt.Printf("acquire error: %v\n", err)
	}
	os.Exit(0)
}

// runProberChild re-executes this test binary as TestHelperLockProber on
// path with env and returns its verdict lines keyed by their first word.
func runProberChild(t *testing.T, path string, env []string) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperLockProber$", "-test.count=1")
	cmd.Env = append(env[:len(env):len(env)], proberHelperEnv+"="+path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("prober child: %v (stdout %q)", err, out)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, _ := strings.Cut(line, " ")
		got[key] = value
	}
	return got
}

// zoneLocaleEnv is this process's environment without the variables that
// set a process's time zone or locale (TZ, LANG, LC_*), plus extra — so a
// child started with it runs in exactly the zone and locale extra names,
// or the system zone and the C locale when extra names none.
func zoneLocaleEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if key == "TZ" || key == "LANG" || strings.HasPrefix(key, "LC_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

// localizingLocale returns an installed locale in which this platform's
// own `ps -o lstart=` prints a start lstartLayouts cannot parse (localized
// day and month names), or "" when no candidate does — the non-C-locale
// rows then have nothing to witness here and skip.
func localizingLocale() string {
	for _, loc := range []string{"de_DE.UTF-8", "de_DE.utf8", "fr_FR.UTF-8", "fr_FR.utf8"} {
		cmd := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(os.Getpid()))
		cmd.Env = zoneLocaleEnv("LC_ALL=" + loc)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if _, perr := parseLstart(strings.TrimSpace(string(out))); perr != nil {
			return loc
		}
	}
	return ""
}

// TestPSLstart_HolderAndProberEnvironmentsAgree is FL-R1's seam-free
// witness (SI-300 as amended): a real holder child and a real prober child,
// each in a time zone and locale of its own, must read the holder's start
// as one value — so the prober judges the live holder held and its Acquire
// gets ErrHeld — whatever zone or locale either runs in. The rows cover a
// POSIX TZ rule string (JST-9: ps honours it, Go's time.Local does not), a
// zoneinfo name both honour (Asia/Tokyo: time.Local then differs from UTC,
// so reading ps's output in time.Local disagrees across processes), and,
// where this platform has one, a non-C locale whose localized ps output
// does not parse (the holder would record its creation time instead, the
// prober could not decide). Before the amendment the JST-9 rows read a
// nine-hour drift and took the live lock over (reviewer witness
// logs/servewitness.log: two live `verdi serve` writers).
func TestPSLstart_HolderAndProberEnvironmentsAgree(t *testing.T) {
	if _, err := psLstart(os.Getpid()); err != nil {
		t.Skipf("ps -o lstart= unavailable/unparseable on this platform: %v", err)
	}
	loc := localizingLocale()
	rows := []struct {
		name           string
		holder, prober []string
		needsLocale    bool
	}{
		{"holder TZ=JST-9, prober plain", []string{"TZ=JST-9"}, nil, false},
		{"holder plain, prober TZ=JST-9", nil, []string{"TZ=JST-9"}, false},
		{"holder TZ=Asia/Tokyo, prober plain", []string{"TZ=Asia/Tokyo"}, nil, false},
		{"holder plain, prober TZ=Asia/Tokyo", nil, []string{"TZ=Asia/Tokyo"}, false},
		{"holder non-C locale, prober plain", []string{"LC_ALL=" + loc}, nil, true},
		{"holder plain, prober non-C locale", nil, []string{"LC_ALL=" + loc}, true},
		{"both non-C locale, holder TZ=JST-9", []string{"LC_ALL=" + loc, "TZ=JST-9"}, []string{"LC_ALL=" + loc}, true},
		{"both non-C locale, prober TZ=JST-9", []string{"LC_ALL=" + loc}, []string{"LC_ALL=" + loc, "TZ=JST-9"}, true},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if r.needsLocale && loc == "" {
				t.Skip("no installed locale localizes ps -o lstart= output on this platform")
			}
			t.Parallel()
			path := filepath.Join(t.TempDir(), "writer.lock")
			holder, stop := startHolderChild(t, path, zoneLocaleEnv(r.holder...))

			info := readLockBody(t, path)
			reading, err := psLstart(holder)
			if err != nil {
				t.Fatalf("ps -o lstart= for the holder child: %v", err)
			}
			if info.PID != holder || info.Start != reading.Unix() {
				t.Errorf("lock body = %+v, want {PID:%d Start:%d}: the holder recorded a start this process does not read (drift %s)", info, holder, reading.Unix(), time.Duration(info.Start-reading.Unix())*time.Second)
			}

			got := runProberChild(t, path, zoneLocaleEnv(r.prober...))
			for _, want := range []struct{ key, value string }{
				{"reading", strconv.FormatInt(info.Start, 10)},
				{"peek", "true"},
				{"inspect", string(LockHeld)},
				{"acquire", "held"},
			} {
				if got[want.key] != want.value {
					t.Errorf("prober %s = %q, want %q (inspect reason %q)", want.key, got[want.key], want.value, got["inspect-reason"])
				}
			}

			taken, err := Acquire(path)
			if err == nil {
				_ = Release(taken, path)
				t.Fatal("this process's Acquire took the live holder's lock over, want *ErrHeld")
			}
			if !errors.As(err, new(*ErrHeld)) {
				t.Fatalf("this process's Acquire = %v, want *ErrHeld", err)
			}
			if err := stop(); err != nil {
				t.Fatalf("holder child exited uncleanly: %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("lock present after the holder released it: %v", err)
			}
		})
	}
}
