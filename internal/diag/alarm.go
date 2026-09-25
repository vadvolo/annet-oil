package diag

import (
	"fmt"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

// AlarmType aliases the generated proto enum.
type AlarmType = diagpb.AlarmType

// alarmNames maps canonical names + common aliases to the enum.
var alarmNames = map[string]diagpb.AlarmType{
	"availability":   diagpb.AlarmType_ALARM_AVAILABILITY,
	"device_down":    diagpb.AlarmType_ALARM_AVAILABILITY,
	"device-down":    diagpb.AlarmType_ALARM_AVAILABILITY,
	"up_down":        diagpb.AlarmType_ALARM_AVAILABILITY,
	"port_optics":    diagpb.AlarmType_ALARM_PORT_OPTICS,
	"port":           diagpb.AlarmType_ALARM_PORT_OPTICS,
	"optics":         diagpb.AlarmType_ALARM_PORT_OPTICS,
	"stp":            diagpb.AlarmType_ALARM_STP,
	"l2_loop":        diagpb.AlarmType_ALARM_STP,
	"ospf":           diagpb.AlarmType_ALARM_OSPF,
	"bgp":            diagpb.AlarmType_ALARM_BGP,
	"bfd":            diagpb.AlarmType_ALARM_BFD,
	"isis":           diagpb.AlarmType_ALARM_ISIS,
	"is-is":          diagpb.AlarmType_ALARM_ISIS,
	"redundancy":     diagpb.AlarmType_ALARM_REDUNDANCY,
	"vrrp":           diagpb.AlarmType_ALARM_REDUNDANCY,
	"hsrp":           diagpb.AlarmType_ALARM_REDUNDANCY,
	"mpls_ldp":       diagpb.AlarmType_ALARM_MPLS_LDP,
	"ldp":            diagpb.AlarmType_ALARM_MPLS_LDP,
	"l2vpn":          diagpb.AlarmType_ALARM_L2VPN,
	"pseudowire":     diagpb.AlarmType_ALARM_L2VPN,
	"cpu_mem":        diagpb.AlarmType_ALARM_CPU_MEM,
	"cpu":            diagpb.AlarmType_ALARM_CPU_MEM,
	"memory":         diagpb.AlarmType_ALARM_CPU_MEM,
	"hardware_env":   diagpb.AlarmType_ALARM_HARDWARE_ENV,
	"hardware":       diagpb.AlarmType_ALARM_HARDWARE_ENV,
	"environment":    diagpb.AlarmType_ALARM_HARDWARE_ENV,
	"restart_uptime": diagpb.AlarmType_ALARM_RESTART_UPTIME,
	"restart":        diagpb.AlarmType_ALARM_RESTART_UPTIME,
	"uptime":         diagpb.AlarmType_ALARM_RESTART_UPTIME,
	"monitoring_gap": diagpb.AlarmType_ALARM_MONITORING_GAP,
}

// ParseAlarm resolves an alarm name (case-insensitive, hyphen/underscore
// tolerant) to the enum, erroring on unknown values.
func ParseAlarm(name string) (diagpb.AlarmType, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.ReplaceAll(key, " ", "_")
	if a, ok := alarmNames[key]; ok {
		return a, nil
	}
	return diagpb.AlarmType_ALARM_UNSPECIFIED, fmt.Errorf("unknown alarm type %q", name)
}

// AlarmName returns the canonical lowercase name for an alarm type.
func AlarmName(a diagpb.AlarmType) string {
	switch a {
	case diagpb.AlarmType_ALARM_AVAILABILITY:
		return "availability"
	case diagpb.AlarmType_ALARM_PORT_OPTICS:
		return "port_optics"
	case diagpb.AlarmType_ALARM_STP:
		return "stp"
	case diagpb.AlarmType_ALARM_OSPF:
		return "ospf"
	case diagpb.AlarmType_ALARM_BGP:
		return "bgp"
	case diagpb.AlarmType_ALARM_BFD:
		return "bfd"
	case diagpb.AlarmType_ALARM_ISIS:
		return "isis"
	case diagpb.AlarmType_ALARM_REDUNDANCY:
		return "redundancy"
	case diagpb.AlarmType_ALARM_MPLS_LDP:
		return "mpls_ldp"
	case diagpb.AlarmType_ALARM_L2VPN:
		return "l2vpn"
	case diagpb.AlarmType_ALARM_CPU_MEM:
		return "cpu_mem"
	case diagpb.AlarmType_ALARM_HARDWARE_ENV:
		return "hardware_env"
	case diagpb.AlarmType_ALARM_RESTART_UPTIME:
		return "restart_uptime"
	case diagpb.AlarmType_ALARM_MONITORING_GAP:
		return "monitoring_gap"
	default:
		return "unspecified"
	}
}
