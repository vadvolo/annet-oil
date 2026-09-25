package diag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"annet-oil/internal/diag/diagpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Target is a device (or probe) the engine can run commands against.
type Target struct {
	Hostname string
	IP       string
	Vendor   string // gnetcli profile (raw inventory vendor)
	Platform string
	Login    string
	Password string
	Port     int
	Telnet   bool // use the telnet streamer instead of ssh
}

// Host returns the address to dial (IP preferred, else hostname).
func (t Target) Host() string {
	if t.IP != "" {
		return t.IP
	}
	return t.Hostname
}

// RunRequest describes one diagnostic run.
type RunRequest struct {
	Device   Target
	Alarm    diagpb.AlarmType
	Port     string // <PORT>
	Peer     string // <PEER>
	CILJ     string // <CILJ> (target)
	Location string // <lok>
	LastHop  string // availability: last traceroute hop before the device
	Extra    map[string]string

	// Availability only: probes to ping FROM and extra anchor targets to ping TO.
	Probes      []Target
	CoreAnchors []string
}

// Config tunes engine behaviour.
type Config struct {
	DefaultTimeoutSec float64       // per-command gnetcli timeout (0 = server default)
	StepBudget        time.Duration // total wall-clock budget (0 = unlimited)
	PingCount         int           // probe ping count (default 4)
}

// Engine runs read-only diagnostic playbooks and returns structured results.
type Engine struct {
	exec DeviceExecutor
	cfg  Config
}

func NewEngine(exec DeviceExecutor, cfg Config) *Engine {
	if cfg.PingCount <= 0 {
		cfg.PingCount = 4
	}
	return &Engine{exec: exec, cfg: cfg}
}

// Run executes the playbook for req.Alarm against req.Device.
func (e *Engine) Run(ctx context.Context, req RunRequest) *diagpb.DiagnosticRun {
	family := ResolveFamily(req.Device.Vendor, req.Device.Platform)
	start := time.Now()

	run := &diagpb.DiagnosticRun{
		Device: &diagpb.DeviceRef{
			Hostname: req.Device.Hostname,
			Ip:       req.Device.IP,
			Vendor:   req.Device.Vendor,
			Platform: req.Device.Platform,
		},
		Alarm: &diagpb.AlarmContext{
			Type:     req.Alarm,
			TypeName: AlarmName(req.Alarm),
			Port:     req.Port,
			Peer:     req.Peer,
			Target:   req.CILJ,
			Location: req.Location,
			Extra:    req.Extra,
		},
		Family:     family,
		FamilyName: FamilyName(family),
		StartedAt:  timestamppb.New(start),
	}

	steps, gap := Playbook(req.Alarm, family)
	if gap != "" {
		run.Gaps = append(run.Gaps, gap)
	}

	var deadline time.Time
	if e.cfg.StepBudget > 0 {
		deadline = start.Add(e.cfg.StepBudget)
	}
	overBudget := func() bool { return !deadline.IsZero() && time.Now().After(deadline) }

	// Availability: run reachability probes first (ping + traceroute from each
	// probe to the device, the last hop, and the core anchors).
	if req.Alarm == diagpb.AlarmType_ALARM_AVAILABILITY {
		for _, ps := range e.probeSteps(ctx, req, &overBudget) {
			run.Steps = append(run.Steps, ps)
		}
	}

	if len(steps) == 0 && gap == "" {
		run.Note = fmt.Sprintf("no playbook for family %s / alarm %s", FamilyName(family), AlarmName(req.Alarm))
	}

	for _, st := range steps {
		cmd, ok := substitute(st.Command, req)
		pbStep := &diagpb.Step{Command: cmd, Category: st.Category}
		switch {
		case !ok:
			pbStep.Status = diagpb.StepStatus_STEP_SKIPPED
			pbStep.Error = "unresolved placeholder (missing alarm field)"
		case overBudget():
			pbStep.Status = diagpb.StepStatus_STEP_SKIPPED
			pbStep.Error = "step budget exceeded"
			run.BudgetExceeded = true
		case !isReadOnly(cmd):
			pbStep.Status = diagpb.StepStatus_STEP_SKIPPED
			pbStep.Error = "command rejected by read-only guard"
		default:
			e.runStep(ctx, req.Device, cmd, family, st.Category, pbStep)
		}
		run.Steps = append(run.Steps, pbStep)
	}

	run.TotalDurationMs = time.Since(start).Milliseconds()
	return run
}

// runStep executes one command against a target and fills the proto step.
func (e *Engine) runStep(ctx context.Context, dev Target, cmd string, family diagpb.Family, category string, out *diagpb.Step) {
	stepStart := time.Now()
	res, err := e.execWithRetry(ctx, dev, cmd)
	out.DurationMs = time.Since(stepStart).Milliseconds()

	if err != nil {
		out.Status = diagpb.StepStatus_STEP_TRANSPORT_ERROR
		out.Error = err.Error()
		return
	}
	out.RawOutput = res.Output
	out.DeviceStatus = res.Status
	out.ErrorCode = res.ErrorCode
	out.Error = res.Error
	if res.Status != 0 || res.ErrorCode != "" {
		out.Status = diagpb.StepStatus_STEP_DEVICE_ERROR
	} else {
		out.Status = diagpb.StepStatus_STEP_OK
	}
	// Parse whatever output we got (partial output on device error is still useful).
	if p := parseCategory(family, category, res.Output); p != nil {
		out.Parsed = p
	}
}

// execWithRetry retries once on transport error (transient session/echo races).
func (e *Engine) execWithRetry(ctx context.Context, dev Target, cmd string) (*ExecOutcome, error) {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		r, err := e.exec.Exec(ctx, dev, cmd)
		if err == nil {
			return &r, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

// probeSteps runs ping+traceroute from each probe to the device/last-hop/anchors.
func (e *Engine) probeSteps(ctx context.Context, req RunRequest, over *func() bool) []*diagpb.Step {
	var out []*diagpb.Step
	targets := []string{req.Device.Host()}
	if req.LastHop != "" {
		targets = append(targets, req.LastHop)
	}
	targets = append(targets, req.CoreAnchors...)

	for _, probe := range req.Probes {
		for _, tgt := range targets {
			if tgt == "" {
				continue
			}
			if (*over)() {
				return out
			}
			pingCmd := fmt.Sprintf("/ping %s count=%d", tgt, e.cfg.PingCount)
			trCmd := fmt.Sprintf("/tool traceroute %s count=1", tgt)
			out = append(out, e.probeOne(ctx, probe, pingCmd, "ping", tgt))
			if (*over)() {
				return out
			}
			out = append(out, e.probeOne(ctx, probe, trCmd, "traceroute", tgt))
		}
	}
	return out
}

func (e *Engine) probeOne(ctx context.Context, probe Target, cmd, category, tgt string) *diagpb.Step {
	step := &diagpb.Step{Command: cmd, Category: category, ProbeHost: probe.Host()}
	if !isReadOnly(cmd) {
		step.Status = diagpb.StepStatus_STEP_SKIPPED
		step.Error = "command rejected by read-only guard"
		return step
	}
	stepStart := time.Now()
	res, err := e.execWithRetry(ctx, probe, cmd)
	step.DurationMs = time.Since(stepStart).Milliseconds()
	if err != nil {
		step.Status = diagpb.StepStatus_STEP_TRANSPORT_ERROR
		step.Error = err.Error()
		return step
	}
	step.RawOutput = res.Output
	step.DeviceStatus = res.Status
	step.ErrorCode = res.ErrorCode
	step.Error = res.Error
	if res.Status != 0 || res.ErrorCode != "" {
		step.Status = diagpb.StepStatus_STEP_DEVICE_ERROR
	} else {
		step.Status = diagpb.StepStatus_STEP_OK
	}
	// Probes are RouterOS; parse ping/traceroute with the routeros parser.
	if p := parseCategory(diagpb.Family_FAMILY_ROUTEROS, category, res.Output); p != nil {
		out := p
		if category == "ping" && out.GetPing() != nil {
			out.GetPing().Target = tgt
		}
		step.Parsed = out
	}
	return step
}

// substitute replaces <PORT>/<PEER>/<CILJ>/<lok> in cmd. It returns ok=false when
// a placeholder remains unresolved (the corresponding alarm field was empty).
func substitute(cmd string, req RunRequest) (string, bool) {
	r := strings.NewReplacer(
		"<PORT>", req.Port,
		"<PEER>", req.Peer,
		"<CILJ>", req.CILJ,
		"<lok>", req.Location,
	)
	out := r.Replace(cmd)
	if strings.Contains(out, "<") && strings.Contains(out, ">") {
		return out, false
	}
	return out, true
}
