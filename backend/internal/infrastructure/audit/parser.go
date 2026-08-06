// Package audit provides auditd log parsing and event correlation.
package audit

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Event represents a parsed audit event.
type Event struct {
	Timestamp  time.Time
	Serial     uint64      // msg=audit(timestamp:serial)
	Type       string      // type=SYSCALL, type=EXECVE, etc.
	ProcessID  uint32      // pid
	ParentPID  uint32      // ppid
	SessionID  uint64      // ses
	UserID     uint32      // uid
	LoginUID   uint32      // auid
	EffectiveUID uint32    // euid
	ProcessName string     // name=
	Command    []string    // argc/argv for execve
	Path       string      // path=
	Directory  string      // dir=
	ReturnCode int         // success=... exit=
	Syscall    int         // syscall=
	ExitCode   int         // exit=
	Arch       string      // arch=
	TTY        string      // tty=
	HostName   string      // hostname=
	Addr       string      // addr=
	Port       int         // lport/rport
	Comm       string      // comm=
	GID        uint32      // gid=
	EGID       uint32      // egid=
	SGID       uint32      // sgid=
	FSGID      uint32      // fsgid=
	Mode       string      // mode=
	OFlag      string      // oflag=
	Key        string      // key=
	Items      int         // items=
}

// ParseEvent parses a single audit log line.
func ParseEvent(line string) (*Event, error) {
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
		return nil, nil // skip empty or comment lines
	}

	// Try JSON format first (auditd 2.8+)
	if strings.HasPrefix(line, "{") {
		return parseJSON(line)
	}

	// Parse traditional format: type=... msg=audit(...)
	event := &Event{}

	// Extract type
	if idx := strings.Index(line, "type="); idx != -1 {
		rest := line[idx+5:]
		if end := strings.IndexAny(rest, " "); end != -1 {
			event.Type = rest[:end]
		} else {
			event.Type = rest
		}
	}

	// Extract msg=audit(timestamp:serial)
	if idx := strings.Index(line, "msg=audit("); idx != -1 {
		rest := line[idx+10:]
		if end := strings.Index(rest, ")"); end != -1 {
			timestampSerial := rest[:end]
			parts := strings.Split(timestampSerial, ":")
			if len(parts) >= 1 {
				// Handle timestamp with microseconds: "1720000000.123"
				tsParts := strings.Split(parts[0], ".")
				var ts int64
				if tsInt, err := strconv.ParseInt(tsParts[0], 10, 64); err == nil {
					ts = tsInt
				}
				// If there's a fractional part, we can use it for nanoseconds (optional)
				if len(tsParts) >= 2 {
					// Just take the integer part for now
				}
				event.Timestamp = time.Unix(ts, 0)
				if len(parts) >= 2 {
					if serial, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
						event.Serial = serial
					}
				}
			}
		}
	}

	// Parse key=value pairs
	event.parseKeyValues(line)

	return event, nil
}

// parseKeyValues extracts key=value pairs from audit line.
func (e *Event) parseKeyValues(line string) {
	// Match key=value patterns
	// Values can be quoted strings or unquoted until whitespace
	re := regexp.MustCompile(`(\w+)=("(?:[^"\\]|\\.)*"|[^"\s]+)`)

	for _, match := range re.FindAllStringSubmatch(line, -1) {
		if len(match) < 3 {
			continue
		}
		key := match[1]
		value := match[2]

		// Remove quotes if present
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}

		switch key {
		case "pid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.ProcessID = uint32(v)
			}
		case "ppid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.ParentPID = uint32(v)
			}
		case "ses":
			if v, err := strconv.ParseUint(value, 10, 64); err == nil {
				e.SessionID = v
			}
		case "uid", "uid2":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.UserID = uint32(v)
			}
		case "auid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.LoginUID = uint32(v)
			}
		case "euid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.EffectiveUID = uint32(v)
			}
		case "name":
			e.Path = value
		case "argc":
			// argc is just a count, argv comes separately
		case "success":
			// success=yes or success=no - used with exit
		case "exit":
			if v, err := strconv.Atoi(value); err == nil {
				e.ExitCode = v
			}
		case "syscall":
			if v, err := strconv.Atoi(value); err == nil {
				e.Syscall = v
			}
		case "arch":
			e.Arch = value
		case "tty":
			e.TTY = value
		case "hostname":
			e.HostName = value
		case "addr", "laddr", "raddr":
			e.Addr = value
		case "lport", "rport":
			if v, err := strconv.Atoi(value); err == nil {
				e.Port = v
			}
		case "comm":
			e.Comm = value
		case "gid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.GID = uint32(v)
			}
		case "egid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.EGID = uint32(v)
			}
		case "sgid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.SGID = uint32(v)
			}
		case "fsgid":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				e.FSGID = uint32(v)
			}
		case "mode":
			e.Mode = value
		case "oflag":
			e.OFlag = value
		case "key":
			e.Key = value
		case "items":
			if v, err := strconv.Atoi(value); err == nil {
				e.Items = v
			}
		case "dir", "cwd":
			e.Directory = value
		case "exe":
			e.ProcessName = value
		}
	}
}

// parseJSON parses JSON format audit logs (auditd 2.8+).
func parseJSON(line string) (*Event, error) {
	var raw struct {
		Timestamp  string `json:"ts"`
		Serial     string `json:"serial"`
		Type       string `json:"type"`
		PID        string `json:"pid"`
		PPID       string `json:"ppid"`
		SessionID  string `json:"ses"`
		UserID     string `json:"uid"`
		LoginUID   string `json:"auid"`
		Path       string `json:"path"`
		Comm       string `json:"comm"`
		Hostname   string `json:"hostname"`
		Addr       string `json:"addr"`
		Port       string `json:"port"`
		ExitCode   string `json:"exit"`
		Syscall    string `json:"syscall"`
		GID        string `json:"gid"`
		EGID       string `json:"egid"`
		Mode       string `json:"mode"`
		Key        string `json:"key"`
		Items      string `json:"items"`
	}

	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	event := &Event{
		Type:        raw.Type,
		Path:        raw.Path,
		Comm:        raw.Comm,
		HostName:    raw.Hostname,
		Addr:        raw.Addr,
		Mode:        raw.Mode,
		Key:         raw.Key,
	}

	if ts, err := strconv.ParseFloat(raw.Timestamp, 64); err == nil {
		event.Timestamp = time.Unix(int64(ts), 0)
	}

	if v, err := strconv.ParseUint(raw.Serial, 10, 64); err == nil {
		event.Serial = v
	}
	if v, err := strconv.ParseUint(raw.PID, 10, 32); err == nil {
		event.ProcessID = uint32(v)
	}
	if v, err := strconv.ParseUint(raw.PPID, 10, 32); err == nil {
		event.ParentPID = uint32(v)
	}
	if v, err := strconv.ParseUint(raw.SessionID, 10, 64); err == nil {
		event.SessionID = v
	}
	if v, err := strconv.ParseUint(raw.UserID, 10, 32); err == nil {
		event.UserID = uint32(v)
	}
	if v, err := strconv.ParseUint(raw.LoginUID, 10, 32); err == nil {
		event.LoginUID = uint32(v)
	}
	if v, err := strconv.Atoi(raw.ExitCode); err == nil {
		event.ExitCode = v
	}
	if v, err := strconv.Atoi(raw.Syscall); err == nil {
		event.Syscall = v
	}
	if v, err := strconv.ParseUint(raw.GID, 10, 32); err == nil {
		event.GID = uint32(v)
	}
	if v, err := strconv.ParseUint(raw.EGID, 10, 32); err == nil {
		event.EGID = uint32(v)
	}
	if v, err := strconv.Atoi(raw.Items); err == nil {
		event.Items = v
	}
	if v, err := strconv.Atoi(raw.Port); err == nil {
		event.Port = v
	}

	return event, nil
}

// Reader provides streaming access to audit log entries.
type Reader struct {
	scanner *bufio.Scanner
	lineNum int
}

// NewReader creates a new audit log reader.
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long audit lines
	const maxScanTokenSize = 64 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	return &Reader{scanner: scanner}
}

// Read reads the next audit event, skipping empty lines and comments.
func (r *Reader) Read() (*Event, error) {
	for r.scanner.Scan() {
		r.lineNum++
		line := r.scanner.Text()
		event, err := ParseEvent(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", r.lineNum, err)
		}
		if event != nil {
			return event, nil
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

// ReadAll reads all events from the audit log file.
func ReadAll(path string) ([]*Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	var events []*Event
	reader := NewReader(file)

	for {
		event, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return events, err
		}
		events = append(events, event)
	}

	return events, nil
}

// SyscallNames maps syscall numbers to names (x86_64).
var SyscallNames = map[int]string{
	0:   "read",
	1:   "write",
	2:   "open",
	3:   "close",
	4:   "stat",
	5:   "fstat",
	6:   "lstat",
	7:   "poll",
	8:   "lseek",
	9:   "mmap",
	10:  "mprotect",
	11:  "munmap",
	12:  "brk",
	13:  "rt_sigaction",
	14:  "rt_sigreturn",
	15:  "ioctl",
	16:  "pread64",
	17:  "pwrite64",
	18:  "readv",
	19:  "writev",
	20:  "access",
	21:  "pipe",
	22:  "select",
	23:  "sched_yield",
	24:  "mremap",
	25:  "msync",
	26:  "mincore",
	27:  "madvise",
	28:  "shmget",
	29:  "shmat",
	30:  "shmctl",
	31:  "dup",
	32:  "dup2",
	33:  "pause",
	34:  "nanosleep",
	35:  "getitimer",
	36:  "alarm",
	37:  "setitimer",
	38:  "getpid",
	39:  "sendfile",
	40:  "socket",
	41:  "connect",
	42:  "accept",
	43:  "sendto",
	44:  "recvfrom",
	45:  "sendmsg",
	46:  "recvmsg",
	47:  "shutdown",
	48:  "bind",
	49:  "listen",
	50:  "getsockname",
	51:  "getpeername",
	52:  "socketpair",
	53:  "setsockopt",
	54:  "getsockopt",
	55:  "clone",
	56:  "fork",
	57:  "vfork",
	58:  "execve",
	59:  "execve",
	60:  "kill",
	61:  "uname",
	62:  "semget",
	63:  "semop",
	64:  "semctl",
	65:  "shmdt",
	66:  "msgget",
	67:  "msgsnd",
	68:  "msgrcv",
	69:  "msgctl",
	70:  "fcntl",
	71:  "flock",
	72:  "fsync",
	73:  "fdatasync",
	74:  "truncate",
	75:  "ftruncate",
	76:  "getdents",
	77:  "getcwd",
	78:  "chdir",
	79:  "fchdir",
	80:  "rename",
	81:  "mkdir",
	82:  "rmdir",
	83:  "creat",
	84:  "link",
	85:  "unlink",
	86:  "symlink",
	87:  "readlink",
	88:  "chmod",
	89:  "fchmod",
	90:  "chown",
	91:  "fchown",
	92:  "lchown",
	93:  "umask",
	94:  "gettimeofday",
	95:  "getrlimit",
	96:  "getrusage",
	97:  "sysinfo",
	98:  "times",
	99:  "ptrace",
	100: "getuid",
	101: "syslog",
	102: "getgid",
	103: "setuid",
	104: "setgid",
	105: "geteuid",
	106: "getegid",
	107: "setpgid",
	108: "getppid",
	109: "getpgrp",
	110: "setsid",
	111: "setreuid",
	112: "setregid",
	113: "getgroups",
	114: "setgroups",
	115: "setresuid",
	116: "getresuid",
	117: "setresgid",
	118: "getresgid",
	119: "getpgid",
	120: "setfsuid",
	121: "setfsgid",
	122: "getsid",
	123: "capget",
	124: "capset",
	125: "rt_sigpending",
	126: "rt_sigtimedwait",
	127: "rt_sigqueueinfo",
	128: "rt_sigsuspend",
	129: "sigaltstack",
	130: "utime",
	131: "mknod",
	132: "uselib",
	133: "personality",
	134: "ustat",
	135: "statfs",
	136: "fstatfs",
	137: "sysfs",
	138: "getpriority",
	139: "setpriority",
	140: "sched_setparam",
	141: "sched_getparam",
	142: "sched_setscheduler",
	143: "sched_getscheduler",
	144: "sched_get_priority_max",
	145: "sched_get_priority_min",
	146: "sched_rr_get_interval",
	147: "mlock",
	148: "munlock",
	149: "mlockall",
	150: "munlockall",
	151: "vhangup",
	152: "modify_ldt",
	153: "pivot_root",
	154: "_sysctl",
	155: "prctl",
	156: "arch_prctl",
	157: "adjtimex",
	158: "setrlimit",
	159: "chroot",
	160: "sync",
	161: "acct",
	162: "settimeofday",
	163: "mount",
	164: "umount2",
	165: "swapon",
	166: "swapoff",
	167: "reboot",
	168: "sethostname",
	169: "setdomainname",
	170: "iopl",
	171: "ioperm",
	172: "init_module",
	173: "delete_module",
	174: "quotactl",
	175: "gettid",
	176: "readahead",
	177: "setxattr",
	178: "lsetxattr",
	179: "fsetxattr",
	180: "getxattr",
	181: "lgetxattr",
	182: "fgetxattr",
	183: "listxattr",
	184: "llistxattr",
	185: "flistxattr",
	186: "removexattr",
	187: "lremovexattr",
	188: "fremovexattr",
	189: "tkill",
	190: "time",
	191: "futex",
	192: "sched_setaffinity",
	193: "sched_getaffinity",
	194: "io_setup",
	195: "io_destroy",
	196: "io_getevents",
	197: "io_submit",
	198: "io_cancel",
	199: "lookup_dcookie",
	200: "epoll_create",
	201: "remap_file_pages",
	202: "set_tid_address",
	203: "timer_create",
	204: "timer_settime",
	205: "timer_gettime",
	206: "timer_getoverrun",
	207: "timer_delete",
	208: "clock_settime",
	209: "clock_gettime",
	210: "clock_getres",
	211: "clock_nanosleep",
	212: "exit_group",
	213: "epoll_wait",
	214: "epoll_ctl",
	215: "tgkill",
	216: "utimes",
	217: "mbind",
	218: "set_mempolicy",
	219: "get_mempolicy",
	220: "mq_open",
	221: "mq_unlink",
	222: "mq_timedsend",
	223: "mq_timedreceive",
	224: "mq_notify",
	225: "mq_getsetattr",
	226: "kexec_load",
	227: "waitid",
	228: "add_key",
	229: "request_key",
	230: "keyctl",
	231: "ioprio_set",
	232: "ioprio_get",
	233: "inotify_init",
	234: "inotify_add_watch",
	235: "inotify_rm_watch",
	236: "migrate_pages",
	237: "openat",
	238: "mkdirat",
	239: "mknodat",
	240: "fchownat",
	241: "futimesat",
	242: "newfstatat",
	243: "unlinkat",
	244: "renameat",
	245: "linkat",
	246: "symlinkat",
	247: "readlinkat",
	248: "fchmodat",
	249: "faccessat",
	250: "pselect6",
	251: "ppoll",
	252: "signalfd",
	253: "timerfd_create",
	254: "eventfd",
	255: "fallocate",
	256: "timerfd_settime",
	257: "openat",
	258: "accept4",
	259: "signalfd4",
	260: "eventfd2",
	261: "epoll_create1",
	262: "dup3",
	263: "pipe2",
	264: "inotify_init1",
	265: "preadv",
	266: "pwritev",
	267: "rt_tgsigqueueinfo",
	268: "perf_event_open",
	269: "recvmmsg",
	270: "fanotify_init",
	271: "fanotify_mark",
	272: "prlimit68",
	273: "name_to_handle_at",
	274: "open_by_handle_at",
	275: "clock_adjtime",
	276: "syncfs",
	277: "sendmmsg",
	278: "setns",
	279: "getcpu",
	280: "process_vm_readv",
	281: "process_vm_writev",
}

// GetSyscallName returns the name for a syscall number.
func GetSyscallName(n int) string {
	if name, ok := SyscallNames[n]; ok {
		return name
	}
	return fmt.Sprintf("syscall_%d", n)
}

// FileOperationTypes returns true if the event type is a file operation.
func FileOperationTypes() map[string]bool {
	return map[string]bool{
		"SYSCALL":    true,
		"PATH":       true,
		"CWD":        true,
		"EXECVE":     true,
		"PROCTITLE":  true,
		"FILE_OPEN":  true,
		"FILE_CREATE": true,
		"FILE_DELETE": true,
		"FILE_MODIFY": true,
		"FILE_ATTR":   true,
		"MMAP":        true,
		"MPROTECT":    true,
	}
}

// IsFileOperation returns true if the event type is a file operation.
func IsFileOperation(eventType string) bool {
	return FileOperationTypes()[eventType]
}

// FilterByTimeRange filters events within a time range.
func FilterByTimeRange(events []*Event, start, end time.Time) []*Event {
	var result []*Event
	for _, e := range events {
		if (e.Timestamp.Equal(start) || e.Timestamp.After(start)) &&
			(e.Timestamp.Equal(end) || e.Timestamp.Before(end)) {
			result = append(result, e)
		}
	}
	return result
}

// FilterByPID filters events by process ID.
func FilterByPID(events []*Event, pid uint32) []*Event {
	var result []*Event
	for _, e := range events {
		if e.ProcessID == pid {
			result = append(result, e)
		}
	}
	return result
}

// FilterByPath filters events by path (substring match).
func FilterByPath(events []*Event, path string) []*Event {
	var result []*Event
	for _, e := range events {
		if strings.Contains(e.Path, path) {
			result = append(result, e)
		}
	}
	return result
}

// FilterByType filters events by type.
func FilterByType(events []*Event, eventType string) []*Event {
	var result []*Event
	for _, e := range events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// FilterByUser filters events by user ID.
func FilterByUser(events []*Event, uid uint32) []*Event {
	var result []*Event
	for _, e := range events {
		if e.UserID == uid || e.LoginUID == uid {
			result = append(result, e)
		}
	}
	return result
}

// ParseExecveArgs parses argc/argv from audit events.
func ParseExecveArgs(events []*Event) []*Event {
	// Build a map of events by serial to find matching EXECVE args
	eventMap := make(map[uint64]*Event)

	for _, e := range events {
		if e.Type == "EXECVE" {
			eventMap[e.Serial] = e
		}
	}

	// Now parse ARGV events and attach to EXECVE
	for _, e := range events {
		if e.Type == "ARGV" {
			if _, ok := eventMap[e.Serial]; ok {
				// ARGV events can be correlated with parent EXECVE via serial
				// This would need more sophisticated parsing
			}
		}
	}

	return events
}

// ParseAuids parses the login UID information.
func ParseAuids(events []*Event) map[uint64]uint32 {
	// Map serial to login UID for correlation
	auids := make(map[uint64]uint32)
	for _, e := range events {
		if e.LoginUID > 0 {
			auids[e.Serial] = e.LoginUID
		}
	}
	return auids
}

// DecodeAuditRecord decodes raw audit record binary format.
func DecodeAuditRecord(data []byte) (map[string]string, error) {
	// Binary audit records are network byte order
	result := make(map[string]string)

	if len(data) < 8 {
		return nil, fmt.Errorf("record too short")
	}

	// Skip header - just extract strings for now
	// Format: 4-byte size, 4-byte magic
	size := binary.BigEndian.Uint32(data[:4])
	magic := binary.BigEndian.Uint32(data[4:8])

	if magic != 0xC00CFEED {
		return nil, fmt.Errorf("invalid audit record magic: %x", magic)
	}

	// Parse name=value pairs
	rest := data[8:size]
	pairs := strings.Split(string(rest), " ")

	for _, pair := range pairs {
		if idx := strings.Index(pair, "="); idx != -1 {
			key := pair[:idx]
			value := pair[idx+1:]
			result[key] = value
		}
	}

	return result, nil
}
